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

type Address int
type SubConn int

type subConnCacheEntry struct {
	sc            SubConn
	cancel        func()
	abortDeleting bool
}

type lbCacheClientConn struct {
	mu            sync.Mutex // L1
	timeout       time.Duration
	subConnCache  map[Address]*subConnCacheEntry
	subConnToAddr map[SubConn]Address
}

func (ccc *lbCacheClientConn) NewSubConn(addrs []Address) SubConn {
	if len(addrs) != 1 {
		return SubConn(1)
	}
	addrWithoutMD := addrs[0]
	ccc.mu.Lock() // L1
	defer ccc.mu.Unlock()
	if entry, ok := ccc.subConnCache[addrWithoutMD]; ok {
		entry.cancel()
		delete(ccc.subConnCache, addrWithoutMD)
		return entry.sc
	}
	scNew := SubConn(1)
	ccc.subConnToAddr[scNew] = addrWithoutMD
	return scNew
}

func (ccc *lbCacheClientConn) RemoveSubConn(sc SubConn) {
	ccc.mu.Lock() // L1
	defer ccc.mu.Unlock()
	addr, ok := ccc.subConnToAddr[sc]
	if !ok {
		return
	}

	if entry, ok := ccc.subConnCache[addr]; ok {
		if entry.sc != sc {
			delete(ccc.subConnToAddr, sc)
		}
		return
	}

	entry := &subConnCacheEntry{
		sc: sc,
	}
	ccc.subConnCache[addr] = entry

	timer := time.AfterFunc(ccc.timeout, func() { // G3
		ccc.mu.Lock() // L1
		defer ccc.mu.Unlock()
		// deadlocks: 0
		if entry.abortDeleting {
			return // Missing unlock
		}
		delete(ccc.subConnToAddr, sc)
		delete(ccc.subConnCache, addr)
	})

	entry.cancel = func() {
		if !timer.Stop() {
			entry.abortDeleting = true
		}
	}
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() { //G1
			done := make(chan struct{})

			// deadlocks: 0
			ccc := &lbCacheClientConn{
				timeout:       time.Nanosecond,
				subConnCache:  make(map[Address]*subConnCacheEntry),
				subConnToAddr: make(map[SubConn]Address),
			}

			sc := ccc.NewSubConn([]Address{Address(1)})
			go func() { // G2
				// deadlocks: 0
				for i := 0; i < 10000; i++ {
					ccc.RemoveSubConn(sc)
					sc = ccc.NewSubConn([]Address{Address(1)})
				}
				close(done)
			}()
			<-done
		}()
	}
}
