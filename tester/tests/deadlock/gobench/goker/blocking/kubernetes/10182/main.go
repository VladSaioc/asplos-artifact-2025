/*
 * Project: kubernetes
 * Issue or PR  : https://github.com/kubernetes/kubernetes/pull/10182
 * Buggy version: 4b990d128a17eea9058d28a3b3688ab8abafbd94
 * fix commit-id: 64ad3e17ad15cd0f9a4fd86706eec1c572033254
 * Flaky: 15/100
 * Description:
 *   This is a lock-channel bug. goroutine 1 is blocked on a lock
 * held by goroutine 3, while goroutine 3 is blocked on sending
 * message to ch, which is read by goroutine 1.
 */
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type statusManager struct {
	podStatusesLock  sync.RWMutex
	podStatusChannel chan bool
}

func (s *statusManager) Start() {
	go func() {
		// deadlocks: x > 0
		for i := 0; i < 2; i++ {
			s.syncBatch()
		}
	}()
}

func (s *statusManager) syncBatch() {
	runtime.Gosched()
	<-s.podStatusChannel
	s.DeletePodStatus()
}

func (s *statusManager) DeletePodStatus() {
	s.podStatusesLock.Lock()
	defer s.podStatusesLock.Unlock()
}

func (s *statusManager) SetPodStatus() {
	s.podStatusesLock.Lock()
	defer s.podStatusesLock.Unlock()
	s.podStatusChannel <- true
}

func NewStatusManager() *statusManager {
	return &statusManager{
		podStatusChannel: make(chan bool),
	}
}

// 	Example of deadlock trace:
//
//	G1 						G2							G3
//	--------------------------------------------------------------------------------
//	s.Start()
//	s.syncBatch()
//							s.SetPodStatus()
//	<-s.podStatusChannel
//							s.podStatusesLock.Lock()
//							s.podStatusChannel <- true
//							s.podStatusesLock.Unlock()
//							return
//	s.DeletePodStatus()
//														s.podStatusesLock.Lock()
//														s.podStatusChannel <- true
//	s.podStatusesLock.Lock()
//	-----------------------------------G1,G3 leak-------------------------------------

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 1000; i++ {
		go func() {
			// deadlocks: 0
			s := NewStatusManager()
			// deadlocks: 0
			go s.Start()
			// deadlocks: x > 0
			go s.SetPodStatus()
			// deadlocks: x > 0
			go s.SetPodStatus()
		}()
	}
}
