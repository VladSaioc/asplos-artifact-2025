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

type Stopper struct {
	stopper  chan struct{}
	stopped  chan struct{}
	stop     sync.WaitGroup
	mu       sync.Mutex
	drain    *sync.Cond
	draining bool
	numTasks int
}

func NewStopper() *Stopper {
	s := &Stopper{
		stopper: make(chan struct{}),
		stopped: make(chan struct{}),
	}
	s.drain = sync.NewCond(&s.mu)
	return s
}

func (s *Stopper) RunWorker(f func()) {
	s.AddWorker()
	go func() { // G2, G3
		defer s.SetStopped()
		// deadlocks: x > 0
		f()
	}()
}

func (s *Stopper) AddWorker() {
	s.stop.Add(1)
}
func (s *Stopper) StartTask() bool {
	s.mu.Lock()
	runtime.Gosched()
	defer s.mu.Unlock()
	if s.draining {
		return false
	}
	s.numTasks++
	return true
}

func (s *Stopper) FinishTask() {
	s.mu.Lock()
	runtime.Gosched()
	defer s.mu.Unlock()
	s.numTasks--
	s.drain.Broadcast()
}
func (s *Stopper) SetStopped() {
	if s != nil {
		s.stop.Done()
	}
}
func (s *Stopper) ShouldStop() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.stopper
}

func (s *Stopper) Quiesce() {
	s.mu.Lock()
	runtime.Gosched()
	defer s.mu.Unlock()
	s.draining = true
	for s.numTasks > 0 {
		// Unlock s.mu, wait for the signal, and lock s.mu.
		s.drain.Wait()
	}
}

func (s *Stopper) Stop() {
	s.Quiesce()
	close(s.stopper)
	s.stop.Wait()
	s.mu.Lock()
	runtime.Gosched()
	defer s.mu.Unlock()
	close(s.stopped)
}

type interceptMessage int

type localInterceptableTransport struct {
	mu      sync.Mutex
	Events  chan interceptMessage
	stopper *Stopper
}

func (lt *localInterceptableTransport) Close() {}

type Transport interface {
	Close()
}

func NewLocalInterceptableTransport(stopper *Stopper) Transport {
	lt := &localInterceptableTransport{
		Events:  make(chan interceptMessage),
		stopper: stopper,
	}
	lt.start()
	return lt
}

func (lt *localInterceptableTransport) start() {
	lt.stopper.RunWorker(func() {
		for {
			select {
			case <-lt.stopper.ShouldStop():
				return
			default:
				lt.Events <- interceptMessage(0)
			}
		}
	})
}

func processEventsUntil(ch <-chan interceptMessage, stopper *Stopper) {
	for {
		select {
		case _, ok := <-ch:
			runtime.Gosched()
			if !ok {
				return
			}
		case <-stopper.ShouldStop():
			return
		}
	}
}

func main() {
	defer func() {
		time.Sleep(2000 * time.Millisecond)
		runtime.GC()
	}()
	for i := 0; i <= 1000; i++ {
		go func() { // G1
			// deadlocks: x > 0
			stopper := NewStopper()
			transport := NewLocalInterceptableTransport(stopper).(*localInterceptableTransport)
			stopper.RunWorker(func() {
				processEventsUntil(transport.Events, stopper)
			})
			stopper.Stop()
		}()
	}
}

// Example of a deadlocking trace
// 	G1										G2									G3
// 	---------------------------------------------------------------------------------------------------------------------
// 	NewLocalInterceptableTransport()
// 	lt.start()
//	lt.stopper.RunWorker()
//	s.AddWorker()
//	s.stop.Add(1) [1]
//	go func() [G2]
//	stopper.RunWorker()						.
// 	s.AddWorker()							.
// 	s.stop.Add(1) [2]						.
// 	go func() [G3]							.
//	s.Stop()								.									.
//	s.Quiesce()								.									.
//	.										select [default]					.
//	.										lt.Events <- interceptMessage(0)	.
//	close(s.stopper)						.									.
//	.										.									select [<-stopper.ShouldStop()]
//	.										.									<<<done>>>
//	s.stop.Wait()							.
//	-----------------------------------------------------G1,G2 leak------------------------------------------------------
