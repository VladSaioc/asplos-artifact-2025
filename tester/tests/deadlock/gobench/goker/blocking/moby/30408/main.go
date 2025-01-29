package main

import (
	"errors"
	"runtime"
	"sync"
	"time"
)

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
		// deadlocks: 100
		p.waitActive()
		close(done)
	}()
	// Also a true positive
	<-done
}
func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			// deadlocks: 100
			p := &Plugin{activateWait: sync.NewCond(&sync.Mutex{})}
			p.activateErr = errors.New("some junk happened")

			testActive(p)
		}()
	}
}
