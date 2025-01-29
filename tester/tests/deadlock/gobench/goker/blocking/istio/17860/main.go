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

type Proxy interface {
	IsLive() bool
}

type TestProxy struct {
	live func() bool
}

func (tp TestProxy) IsLive() bool {
	if tp.live == nil {
		return true
	}
	return tp.live()
}

type Agent interface {
	Run(ctx context.Context)
	Restart()
}

type exitStatus int

type agent struct {
	proxy        Proxy
	mu           *sync.Mutex
	statusCh     chan exitStatus
	currentEpoch int
	activeEpochs map[int]struct{}
}

func (a *agent) Run(ctx context.Context) {
	for {
		select {
		case status := <-a.statusCh:
			a.mu.Lock()
			delete(a.activeEpochs, int(status))
			active := len(a.activeEpochs)
			a.mu.Unlock()
			if active == 0 {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (a *agent) Restart() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.waitUntilLive()
	a.currentEpoch++
	a.activeEpochs[a.currentEpoch] = struct{}{}

	// deadlocks: x > 0
	go a.runWait(a.currentEpoch)
}

func (a *agent) runWait(epoch int) {
	a.statusCh <- exitStatus(epoch)
}

func (a *agent) waitUntilLive() {
	if len(a.activeEpochs) == 0 {
		return
	}

	interval := time.NewTicker(30 * time.Nanosecond)
	timer := time.NewTimer(100 * time.Nanosecond)
	defer func() {
		interval.Stop()
		timer.Stop()
	}()

	if a.proxy.IsLive() {
		return
	}

	for {
		select {
		case <-timer.C:
			return
		case <-interval.C:
			if a.proxy.IsLive() {
				return
			}
		}
	}
}

func NewAgent(proxy Proxy) Agent {
	return &agent{
		proxy:        proxy,
		mu:           &sync.Mutex{},
		statusCh:     make(chan exitStatus),
		activeEpochs: make(map[int]struct{}),
	}
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			neverLive := func() bool {
				return false
			}

			a := NewAgent(TestProxy{live: neverLive})
			go func() { a.Run(ctx) }()

			a.Restart()
			go a.Restart()

			time.Sleep(200 * time.Nanosecond)
		}()
	}
}
