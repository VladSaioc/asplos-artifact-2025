package main

import (
	"runtime"
	"sync"
	"time"
)

type Source interface {
	Start()
	Stop()
}

type fsSource struct {
	donec chan struct{}
}

func (s *fsSource) Start() {
	go func() {
		// deadlocks: 1
		for {
			select {
			case <-s.donec:
				return
			}
		}
	}()
}

func (s *fsSource) Stop() {
	close(s.donec)
	s.donec = nil
}

func New() Source {
	return &fsSource{
		donec: make(chan struct{}),
	}
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s := New()
		s.Start()
		s.Stop()
		time.Sleep(5 * time.Millisecond)
	}()
	wg.Wait()
}
