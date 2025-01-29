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

type remoteLock struct {
	sync.RWMutex                        // L1
	m            map[string]*sync.Mutex // L2
}

func (l *remoteLock) URLLock(url string) {
	l.Lock() // L1
	if _, ok := l.m[url]; !ok {
		l.m[url] = &sync.Mutex{}
	}
	l.m[url].Lock() // L2
	runtime.Gosched()
	l.Unlock() // L1
	// runtime.Gosched()
}

func (l *remoteLock) URLUnlock(url string) {
	l.RLock()         // L1
	defer l.RUnlock() // L1
	if um, ok := l.m[url]; ok {
		um.Unlock() // L2
	}
}

func resGetRemote(remoteURLLock *remoteLock, url string) error {
	remoteURLLock.URLLock(url)
	defer func() { remoteURLLock.URLUnlock(url) }()

	return nil
}

func main() {
	defer func() {
		time.Sleep(4 * time.Second)
		runtime.GC()
	}()

	for i := 0; i < 10; i++ {
		go func() { // G1
			// deadlocks: x > 0
			url := "http://Foo.Bar/foo_Bar-Foo"
			remoteURLLock := &remoteLock{m: make(map[string]*sync.Mutex)}
			for range []bool{false, true} {
				var wg sync.WaitGroup
				for i := 0; i < 100; i++ {
					wg.Add(1)
					go func(gor int) { // G2
						// deadlocks: x > 0
						defer wg.Done()
						for j := 0; j < 200; j++ {
							err := resGetRemote(remoteURLLock, url)
							if err != nil {
								fmt.Errorf("Error getting resource content: %s", err)
							}
							time.Sleep(300 * time.Nanosecond)
						}
					}(i)
				}
				wg.Wait()
			}
		}()
	}
}

// Example of deadlocking trace:
//
// 	G1								G2									G3
// 	------------------------------------------------------------------------------------------------
//	wg.Add(1) [W1: 1]
//	go func() [G2]
//	go func() [G3]
//	.								resGetRemote()
//	.								remoteURLLock.URLLock(url)
//	.								l.Lock() [L1]
//	.								l.m[url] = &sync.Mutex{} [L2]
//	.								l.m[url].Lock()	[L2]
//	.								l.Unlock()	[L1]
//	.								.									resGetRemote()
//	.								.									remoteURLLock.URLLock(url)
//	.								.									l.Lock()	[L1]
//	.								.									l.m[url].Lock()	[L2]
//	.								remoteURLLock.URLUnlock(url)
//	.								l.RLock()	[L1]
//	...
//	wg.Wait() [W1]
//	----------------------------------------G1,G2,G3 leak-------------------------------------------
