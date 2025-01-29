/*
 * Project: kubernetes
 * Issue or PR  : https://github.com/kubernetes/kubernetes/pull/1321
 * Buggy version: 9cd0fc70f1ca852c903b18b0933991036b3b2fa1
 * fix commit-id: 435e0b73bb99862f9dedf56a50260ff3dfef14ff
 * Flaky: 1/100
 * Description:
 *   This is a lock-channel bug. The first goroutine invokes
 * distribute() function. distribute() function holds m.lock.Lock(),
 * while blocking at sending message to w.result. The second goroutine
 * invokes stopWatching() funciton, which can unblock the first
 * goroutine by closing w.result. However, in order to close w.result,
 * stopWatching() function needs to acquire m.lock.Lock() firstly.
 *   The fix is to introduce another channel and put receive message
 * from the second channel in the same select as the w.result. Close
 * the second channel can unblock the first goroutine, while no need
 * to hold m.lock.Lock().
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

type muxWatcher struct {
	result chan struct{}
	m      *Mux
	id     int64
}

func (mw *muxWatcher) Stop() {
	mw.m.stopWatching(mw.id)
}

type Mux struct {
	lock     sync.Mutex
	watchers map[int64]*muxWatcher
}

func NewMux() *Mux {
	m := &Mux{
		watchers: map[int64]*muxWatcher{},
	}
	// deadlocks: x > 0
	go m.loop() // G2
	return m
}

func (m *Mux) Watch() *muxWatcher {
	mw := &muxWatcher{
		result: make(chan struct{}),
		m:      m,
		id:     int64(len(m.watchers)),
	}
	m.watchers[mw.id] = mw
	runtime.Gosched()
	return mw
}

func (m *Mux) loop() {
	for i := 0; i < 100; i++ {
		m.distribute()
	}
}

func (m *Mux) distribute() {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, w := range m.watchers {
		w.result <- struct{}{}
		runtime.Gosched()
	}
}

func (m *Mux) stopWatching(id int64) {
	m.lock.Lock()
	defer m.lock.Unlock()
	w, ok := m.watchers[id]
	if !ok {
		return
	}
	delete(m.watchers, id)
	close(w.result)
}

func testMuxWatcherClose() {
	// deadlocks: x > 0
	m := NewMux()
	m.watchers[m.Watch().id].Stop()
}

///
/// G1 							G2
/// testMuxWatcherClose()
/// NewMux()
/// 							m.loop()
/// 							m.distribute()
/// 							m.lock.Lock()
/// 							w.result <- true
/// w := m.Watch()
/// w.Stop()
/// mw.m.stopWatching()
/// m.lock.Lock()
/// ---------------G1,G2 deadlock---------------
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
				go testMuxWatcherClose() // G1
			}
		}
	}()
}
