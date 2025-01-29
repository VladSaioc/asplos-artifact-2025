package main

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type Manifest struct {
	Implements []string
}

type Plugin struct {
	activateWait *sync.Cond
	activateErr  error
	Manifest     *Manifest
}

func (p *Plugin) waitActive() error {
	p.activateWait.L.Lock()
	for !p.activated() {
		p.activateWait.Wait()
	}
	p.activateWait.L.Unlock()
	return p.activateErr
}

func (p *Plugin) activated() bool {
	return p.Manifest != nil
}

func testActive(p *Plugin) {
	done := make(chan struct{})
	go func() {
		// deadlocks: x > 0
		p.waitActive()
		close(done)
	}()
	// Also a true positive
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
					p := &Plugin{activateWait: sync.NewCond(&sync.Mutex{})}
					p.activateErr = errors.New("some junk happened")

					testActive(p)
				}()
			}
		}
	}()
}
