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
	go func() {
		// deadlocks: 0
		defer s.SetStopped()
		f()
	}()
}

func (s *Stopper) RunWorker2(f func()) {
	s.AddWorker()
	go func() {
		// deadlocks: 0
		defer s.SetStopped()
		f()
	}()
}

func (s *Stopper) AddWorker() {
	s.stop.Add(1)
}
func (s *Stopper) StartTask() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.draining {
		return false
	}
	s.numTasks++
	return true
}

func (s *Stopper) FinishTask() {
	s.mu.Lock()
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
				// FIX: Guard send in if
				if lt.stopper.StartTask() {
					lt.Events <- interceptMessage(0) //@ releases(g1)
					lt.stopper.FinishTask()
				}
			}
		}
	})
}

func processEventsUntil(ch <-chan interceptMessage, stopper *Stopper) {
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-stopper.ShouldStop():
			return
		}
	}
}

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
					// deadlocks: 0
					stopper := NewStopper()
					transport := NewLocalInterceptableTransport(stopper).(*localInterceptableTransport)
					stopper.RunWorker2(func() {
						processEventsUntil(transport.Events, stopper)
					})
					stopper.Stop()
				}()
			}
		}
	}()
}
