package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type Stopper struct {
	stopper  chan struct{}
	stop     sync.WaitGroup
	mu       sync.Mutex
	draining int32
	drain    sync.WaitGroup
}

func (s *Stopper) AddWorker() {
	s.stop.Add(1)
}

func (s *Stopper) ShouldStop() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.stopper
}

func (s *Stopper) SetStopped() {
	if s != nil {
		s.stop.Done()
	}
}

func (s *Stopper) Quiesce() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.draining = 1
	s.drain.Wait()
	s.draining = 0
}

func (s *Stopper) Stop() {
	s.mu.Lock() // L1
	defer s.mu.Unlock()
	atomic.StoreInt32(&s.draining, 1)
	s.drain.Wait()
	close(s.stopper)
	s.stop.Wait()
}

func (s *Stopper) StartTask() bool {
	if atomic.LoadInt32(&s.draining) == 0 {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.drain.Add(1)
		return true
	}
	return false
}

func NewStopper() *Stopper {
	return &Stopper{
		stopper: make(chan struct{}),
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

			for i := 0; i <= 100; i++ {
				go func() { // G1
					// deadlocks: x > 0
					var stoppers []*Stopper
					for i := 0; i < 2; i++ {
						stoppers = append(stoppers, NewStopper())
					}

					for i := range stoppers {
						s := stoppers[i]
						s.AddWorker()
						go func() { // G2
							// deadlocks: x > 0
							s.StartTask()
							<-s.ShouldStop()
							s.SetStopped()
						}()
					}

					done := make(chan struct{})
					go func() { // G3
						// deadlocks: x > 0
						for _, s := range stoppers {
							s.Quiesce()
						}
						for _, s := range stoppers {
							s.Stop()
						}
						close(done)
					}()

					<-done
				}()
			}
		}
	}()
}

//	Example deadlock trace:
//
//	G1						G2.0						G2.1						G2.2						G3
//	---------------------------------------------------------------------------------------------------------------------
//	s[0].stop.Add(1) [1]
//	go func() [G2.0]
//	s[1].stop.Add(1) [1]	.
//	go func() [G2.1]		.
//	s[2].stop.Add(1) [1]	.							.
//	go func() [G2.2]		.							.
//	go func() [G3]			.							.							.
//	<-done					.							.							.							.
//	.						s[0].StartTask()			.							.							.
//	.						s[0].draining == 0			.							.							.
//	.						.							s[1].StartTask()			.							.
//	.						.							s[1].draining == 0			.							.
//	.						.							.							s[2].StartTask()			.
//	.						.							.							s[2].draining == 0			.
//	.						.							.							.							s[0].Quiesce()
//	.						.							.							.							s[0].mu.Lock() [L1[0]]
//	.						s[0].mu.Lock() [L1[0]]		.							.							.
//	.						s[0].drain.Add(1) [1]		.							.							.
//	.						s[0].mu.Unlock() [L1[0]]	.							.							.
//	.						<-s[0].ShouldStop()			.							.							.
//	.						.							.							.							s[0].draining = 1
//	.						.							.							.							s[0].drain.Wait()
//	.						.							s[0].mu.Lock() [L1[1]]		.							.
//	.						.							s[1].drain.Add(1) [1]		.							.
//	.						.							s[1].mu.Unlock() [L1[1]]	.							.
//	.						.							<-s[1].ShouldStop()			.							.
//	.						.							.							s[2].mu.Lock() [L1[2]]		.
//	.						.							.							s[2].drain.Add() [1]		.
//	.						.							.							s[2].mu.Unlock() [L1[2]]	.
//	.						.							.							<-s[2].ShouldStop()			.
//	----------------------------------------------------G1, G2.[0..2], G3 leak------------------------------------------------
