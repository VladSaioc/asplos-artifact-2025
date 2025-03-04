/*
 * Project: cockroach
 * Issue or PR  : https://github.com/cockroachdb/cockroach/pull/10214
 * Buggy version: 7207111aa3a43df0552509365fdec741a53f873f
 * fix commit-id: 27e863d90ab0660494778f1c35966cc5ddc38e32
 * Flaky: 3/100
 * Description: This deadlock is caused by different order when acquiring
 * coalescedMu.Lock() and raftMu.Lock(). The fix is to refactor sendQueuedHeartbeats()
 * so that cockroachdb can unlock coalescedMu before locking raftMu.
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
	coalescedMu struct {
		sync.Mutex
		heartbeatResponses []int
	}
	mu struct {
		replicas map[int]*Replica
	}
}

func (s *Store) sendQueuedHeartbeats() {
	s.coalescedMu.Lock() // LockA acquire
	runtime.Gosched()
	defer s.coalescedMu.Unlock()
	for i := 0; i < len(s.coalescedMu.heartbeatResponses); i++ {
		s.sendQueuedHeartbeatsToNode() // LockB
	}
	// LockA release
}

func (s *Store) sendQueuedHeartbeatsToNode() {
	for i := 0; i < len(s.mu.replicas); i++ {
		r := s.mu.replicas[i]
		r.reportUnreachable() // LockB
	}
}

type Replica struct {
	raftMu sync.Mutex
	mu     sync.Mutex
	store  *Store
}

func (r *Replica) reportUnreachable() {
	r.raftMu.Lock() // LockB acquire
	runtime.Gosched()
	//+time.Sleep(time.Nanosecond)
	defer r.raftMu.Unlock()
	// LockB release
}

func (r *Replica) tick() {
	r.raftMu.Lock() // LockB acquire
	runtime.Gosched()
	defer r.raftMu.Unlock()
	r.tickRaftMuLocked()
	// LockB release
}

func (r *Replica) tickRaftMuLocked() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.maybeQuiesceLocked() {
		return
	}
}
func (r *Replica) maybeQuiesceLocked() bool {
	for i := 0; i < 2; i++ {
		if !r.maybeCoalesceHeartbeat() {
			return true
		}
	}
	return false
}
func (r *Replica) maybeCoalesceHeartbeat() bool {
	msgtype := uintptr(unsafe.Pointer(r)) % 3
	switch msgtype {
	case 0, 1, 2:
		r.store.coalescedMu.Lock() // LockA acquire
	default:
		return false
	}
	r.store.coalescedMu.Unlock() // LockA release
	return true
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	for i := 0; i < 1000; i++ {
		go func() {
			store := &Store{}
			responses := &store.coalescedMu.heartbeatResponses
			*responses = append(*responses, 1, 2)
			store.mu.replicas = make(map[int]*Replica)

			rp1 := &Replica{
				store: store,
			}
			rp2 := &Replica{
				store: store,
			}
			store.mu.replicas[0] = rp1
			store.mu.replicas[1] = rp2

			go func() {
				// deadlocks: x > 0
				store.sendQueuedHeartbeats()
			}()

			go func() {
				// deadlocks: x > 0
				rp1.tick()
			}()

		}()
	}
}
