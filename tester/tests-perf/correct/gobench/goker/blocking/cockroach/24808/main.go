package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type Compactor struct {
	ch chan struct{}
}

type Stopper struct {
	stop    sync.WaitGroup
	stopper chan struct{}
}

func (s *Stopper) RunWorker(ctx context.Context, f func(context.Context)) {
	s.stop.Add(1)
	go func() {
		// deadlocks: 0
		defer s.stop.Done()
		f(ctx)
	}()
}

func (s *Stopper) ShouldStop() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.stopper
}

func (s *Stopper) Stop() {
	close(s.stopper)
}

func NewStopper() *Stopper {
	s := &Stopper{
		stopper: make(chan struct{}),
	}
	return s
}

func NewCompactor() *Compactor {
	return &Compactor{ch: make(chan struct{}, 1)}
}

func (c *Compactor) Start(ctx context.Context, stopper *Stopper) {
	select {
	case c.ch <- struct{}{}:
	default:
	}
	stopper.RunWorker(ctx, func(ctx context.Context) {
		for {
			select {
			case <-stopper.ShouldStop():
				return
			case <-c.ch:
			}
		}
	})
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
					defer stopper.Stop()

					compactor := NewCompactor()
					compactor.ch <- struct{}{}

					compactor.Start(context.Background(), stopper)
				}()
			}
		}
	}()
}
