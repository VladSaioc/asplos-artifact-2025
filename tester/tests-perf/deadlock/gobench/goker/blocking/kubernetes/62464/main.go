/*
 * Project: kubernetes
 * Issue or PR  : https://github.com/kubernetes/kubernetes/pull/62464
 * Buggy version: a048ca888ad27367b1a7b7377c67658920adbf5d
 * fix commit-id: c1b19fce903675b82e9fdd1befcc5f5d658bfe78
 * Flaky: 8/100
 * Description:
 *   This is another example for recursive read lock bug. It has
 * been noticed by the go developers that RLock should not be
 * recursively used in the same thread.
 */

package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type State interface {
	GetCPUSetOrDefault()
	GetCPUSet() bool
	GetDefaultCPUSet()
	SetDefaultCPUSet()
}

type stateMemory struct {
	sync.RWMutex
}

func (s *stateMemory) GetCPUSetOrDefault() {
	s.RLock()
	defer s.RUnlock()
	if ok := s.GetCPUSet(); ok {
		return
	}
	s.GetDefaultCPUSet()
}

func (s *stateMemory) GetCPUSet() bool {
	runtime.Gosched()
	s.RLock()
	defer s.RUnlock()

	if rand.Intn(10) > 5 {
		return true
	}
	return false
}

func (s *stateMemory) GetDefaultCPUSet() {
	s.RLock()
	defer s.RUnlock()
}

func (s *stateMemory) SetDefaultCPUSet() {
	s.Lock()
	runtime.Gosched()
	defer s.Unlock()
}

type staticPolicy struct{}

func (p *staticPolicy) RemoveContainer(s State) {
	s.GetDefaultCPUSet()
	s.SetDefaultCPUSet()
}

type manager struct {
	state *stateMemory
}

func (m *manager) reconcileState() {
	m.state.GetCPUSetOrDefault()
}

func NewPolicyAndManager() (*staticPolicy, *manager) {
	s := &stateMemory{}
	m := &manager{s}
	p := &staticPolicy{}
	return p, m
}

///
/// G1 									G2
/// m.reconcileState()
/// m.state.GetCPUSetOrDefault()
/// s.RLock()
/// s.GetCPUSet()
/// 									p.RemoveContainer()
/// 									s.GetDefaultCPUSet()
/// 									s.SetDefaultCPUSet()
/// 									s.Lock()
/// s.RLock()
/// ---------------------G1,G2 deadlock---------------------
///

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
				go func() {
					p, m := NewPolicyAndManager()
					// deadlocks: x > 0
					go m.reconcileState()
					// deadlocks: x > 0
					go p.RemoveContainer(m.state)
				}()
			}
		}
	}()
}
