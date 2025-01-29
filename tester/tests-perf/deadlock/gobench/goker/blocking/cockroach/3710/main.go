/*
 * Project: cockroach
 * Issue or PR  : https://github.com/cockroachdb/cockroach/pull/3710
 * Buggy version: 4afdd4860fd7c3bd9e92489f84a95e5cc7d11a0d
 * fix commit-id: cb65190f9caaf464723e7d072b1f1b69a044ef7b
 * Flaky: 2/100
 * Description: This deadlock is casued by acquiring a RLock twice in a call chain.
 * ForceRaftLogScanAndProcess(acquire s.mu.RLock()) ->MaybeAdd()->shouldQueue()->
 * getTruncatableIndexes()->RaftStatus(acquire s.mu.Rlock())
 */

package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

func init() {
	fmt.Println("Starting run...")
}

type Store struct {
	raftLogQueue *baseQueue
	replicas     map[int]*Replica

	mu struct {
		sync.RWMutex
	}
}

func (s *Store) ForceRaftLogScanAndProcess() {
	s.mu.RLock()
	runtime.Gosched()
	for _, r := range s.replicas {
		s.raftLogQueue.MaybeAdd(r)
	}
	s.mu.RUnlock()
}

func (s *Store) RaftStatus() {
	s.mu.RLock()
	defer s.mu.RUnlock()
}

func (s *Store) processRaft() {
	go func() {
		// deadlocks: x > 0
		for {
			var replicas []*Replica
			s.mu.Lock()
			for _, r := range s.replicas {
				replicas = append(replicas, r)
			}
			s.mu.Unlock()
			break
		}
	}()
}

type Replica struct {
	store *Store
}

type baseQueue struct {
	sync.Mutex
	impl *raftLogQueue
}

func (bq *baseQueue) MaybeAdd(repl *Replica) {
	bq.Lock()
	defer bq.Unlock()
	bq.impl.shouldQueue(repl)
}

type raftLogQueue struct{}

func (*raftLogQueue) shouldQueue(r *Replica) {
	getTruncatableIndexes(r)
}

func getTruncatableIndexes(r *Replica) {
	r.store.RaftStatus()
}

func NewStore() *Store {
	rlq := &raftLogQueue{}
	bq := &baseQueue{impl: rlq}
	store := &Store{
		raftLogQueue: bq,
		replicas:     make(map[int]*Replica),
	}
	r1 := &Replica{store}
	r2 := &Replica{store}

	makeKey := func(r *Replica) int {
		return int((uintptr(unsafe.Pointer(r)) >> 1) % 7)
	}
	store.replicas[makeKey(r1)] = r1
	store.replicas[makeKey(r2)] = r2

	return store
}

/// G1 										G2
/// store.ForceRaftLogScanAndProcess()
/// s.mu.RLock()
/// s.raftLogQueue.MaybeAdd()
/// bq.impl.shouldQueue()
/// getTruncatableIndexes()
/// r.store.RaftStatus()
/// 										store.processRaft()
/// 										s.mu.Lock()
/// s.mu.RLock()
/// ----------------------G1,G2 deadlock---------------------

func monitor() {
	var mem = runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	fmt.Println("Final goroutine count:", runtime.NumGoroutine())
}

func main() {
	defer func() {
		time.Sleep(time.Minute / 2)
		runtime.GC()

		monitor()
	}()

	go func() {
		for {
			time.Sleep(time.Second / 2)

			for i := 0; i < 100; i++ {
				store := NewStore()
				// deadlocks: x > 0
				go store.ForceRaftLogScanAndProcess() // G1
				go store.processRaft()
			}
		}
	}()
}
