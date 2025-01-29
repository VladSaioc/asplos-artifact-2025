/*
 * Project: etcd
 * Issue or PR  : https://github.com/etcd-io/etcd/commit/7618fdd1d642e47cac70c03f637b0fd798a53a6e
 * Buggy version: 377f19b0031f9c0aafe2aec28b6f9019311f52f9
 * fix commit-id: 7618fdd1d642e47cac70c03f637b0fd798a53a6e
 * Flaky: 9/100
 */
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

type watchBroadcast struct{}

type watchBroadcasts struct {
	mu      sync.Mutex
	updatec chan *watchBroadcast
	donec   chan struct{}
}

func newWatchBroadcasts() *watchBroadcasts {
	wbs := &watchBroadcasts{
		updatec: make(chan *watchBroadcast, 1),
		donec:   make(chan struct{}),
	}
	go func() { // G2
		// deadlocks: 0
		defer close(wbs.donec)
		for wb := range wbs.updatec {
			wbs.coalesce(wb)
		}
	}()
	return wbs
}

func (wbs *watchBroadcasts) coalesce(wb *watchBroadcast) {
	wbs.mu.Lock()
	wbs.mu.Unlock()
}

func (wbs *watchBroadcasts) stop() {
	wbs.mu.Lock()
	close(wbs.updatec)
	wbs.mu.Unlock()
	<-wbs.donec
}

func (wbs *watchBroadcasts) update(wb *watchBroadcast) {
	select {
	case wbs.updatec <- wb:
	default:
	}
}

///
/// G1						G2					G3
/// newWatchBroadcasts()
///	wbs.update()
/// wbs.updatec <-
/// return
///							<-wbs.updatec
///							wbs.coalesce()
///												wbs.stop()
///												wbs.mu.Lock()
///												close(wbs.updatec)
///												<-wbs.donec
///							wbs.mu.Lock()
///---------------------G2,G3 deadlock-------------------------
///

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
					wbs := newWatchBroadcasts() // G1
					wbs.update(&watchBroadcast{})
					// deadlocks: 0
					go wbs.stop() // G3
				}()
			}
		}
	}()
}
