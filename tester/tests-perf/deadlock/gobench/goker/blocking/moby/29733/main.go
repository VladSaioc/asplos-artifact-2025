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

type Plugin struct {
	activated    bool
	activateWait *sync.Cond
}

type plugins struct {
	sync.Mutex
	plugins map[int]*Plugin
}

func (p *Plugin) waitActive() {
	p.activateWait.L.Lock()
	for !p.activated {
		p.activateWait.Wait()
	}
	p.activateWait.L.Unlock()
}

type extpointHandlers struct {
	sync.RWMutex
	extpointHandlers map[int]struct{}
}

var ()

func Handle(storage plugins, handlers extpointHandlers) {
	handlers.Lock()
	for _, p := range storage.plugins {
		p.activated = false
	}
	handlers.Unlock()
}

func testActive(p *Plugin) {
	done := make(chan struct{})
	go func() {
		// deadlocks: x > 0
		p.waitActive()
		close(done)
	}()
	<-done
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
					// deadlocks: x > 0
					storage := plugins{plugins: make(map[int]*Plugin)}
					handlers := extpointHandlers{extpointHandlers: make(map[int]struct{})}

					p := &Plugin{activateWait: sync.NewCond(&sync.Mutex{})}
					storage.plugins[0] = p

					testActive(p)
					Handle(storage, handlers)
					testActive(p)
				}()
			}
		}
	}()
}
