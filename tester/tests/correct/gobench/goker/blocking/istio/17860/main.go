package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type Proxy interface {
	Run(int) error
	IsLive() bool
}

type TestProxy struct {
	run  func(int) error
	live func() bool
}

func (tp TestProxy) IsLive() bool {
	if tp.live == nil {
		return true
	}
	return tp.live()
}

func (tp TestProxy) Run(epoch int) error {
	if tp.run == nil {
		return nil
	}
	return tp.run(epoch)
}

type Agent interface {
	Run(ctx context.Context)
	Restart(ctx context.Context)
}

type exitStatus int

type agent struct {
	proxy        Proxy
	restartMutex sync.Mutex // L1
	mutex        sync.Mutex // L2
	statusCh     chan exitStatus
	currentEpoch int
	activeEpochs map[int]struct{}
}

func (a *agent) Run(ctx context.Context) {
	for {
		select {
		case status := <-a.statusCh:
			a.mutex.Lock() // L2
			delete(a.activeEpochs, int(status))
			a.mutex.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (a *agent) Restart(ctx context.Context) {
	// Only allow one restart to execute at a time.
	a.restartMutex.Lock() // L1
	defer a.restartMutex.Unlock()

	// Protect access to internal state
	a.mutex.Lock()

	hasActiveEpoch := len(a.activeEpochs) > 0
	activeEpoch := a.currentEpoch

	// Increment the latest running epoch
	a.currentEpoch++
	a.activeEpochs[a.currentEpoch] = struct{}{}

	// Unlock before the wait to avoid delaying envoy exit logic
	a.mutex.Unlock() // L2

	// Wait for the previous epoch to go live (if one exists) before performing hot restart.
	if hasActiveEpoch {
		a.waitUntilLive(activeEpoch)
	}

	// deadlocks: 0
	go a.runWait(a.currentEpoch, ctx) // G3
}

func (a *agent) runWait(epoch int, ctx context.Context) {
	_ = a.proxy.Run(epoch)
	select {
	case a.statusCh <- exitStatus(epoch):
	case <-ctx.Done():
	}
}

func (a *agent) isActive(epoch int) bool {
	a.mutex.Lock() // L2
	defer a.mutex.Unlock()
	_, ok := a.activeEpochs[epoch]
	return ok
}

func (a *agent) waitUntilLive(epoch int) {
	if len(a.activeEpochs) == 0 {
		return
	}

	interval := time.NewTicker(30 * time.Nanosecond)
	timer := time.NewTimer(100 * time.Nanosecond)

	isDone := func() bool {
		if !a.isActive(epoch) {
			return true
		}

		return a.proxy.IsLive()
	}

	defer func() {
		interval.Stop()
		timer.Stop()
	}()

	if isDone() {
		return
	}

	for {
		select {
		case <-timer.C:
			return
		case <-interval.C:
			if isDone() {
				return
			}
		}
	}
}

func NewAgent(proxy Proxy) Agent {
	return &agent{
		proxy:        proxy,
		statusCh:     make(chan exitStatus, 1),
		activeEpochs: make(map[int]struct{}),
	}
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 1; i++ {
		go func() { // G1
			// deadlocks: 0
			ctx, cancel := context.WithCancel(context.Background())

			epoch0Exit := make(chan error, 1)
			epoch1Started := make(chan struct{}, 1)
			start := func(epoch int) error {
				switch epoch {
				case 0:
					// The first epoch just waits for the exit error.
					return <-epoch0Exit
				case 1:
					// Indicate that the second epoch was started.
					close(epoch1Started)
				}
				<-ctx.Done()
				return nil
			}
			neverLive := func() bool {
				return false
			}

			a := NewAgent(TestProxy{
				run:  start,
				live: neverLive,
			})
			go func() { // G2
				// deadlocks: 0
				a.Run(ctx)
			}()

			a.Restart(ctx)
			// deadlocks: 0
			go a.Restart(ctx) // G4

			// Trigger the first epoch to exit
			epoch0Exit <- errors.New("fake")

			<-epoch1Started
			// Started

			cancel()
		}()
	}
}
