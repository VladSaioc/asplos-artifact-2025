/*
 * Project: moby
 * Issue or PR  : https://github.com/moby/moby/pull/7559
 * Buggy version: 64579f51fcb439c36377c0068ccc9a007b368b5a
 * fix commit-id: 6cbb8e070d6c3a66bf48fbe5cbf689557eee23db
 * Flaky: 100/100
 */
package main

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type UDPProxy struct {
	connTrackLock sync.Mutex
}

func (proxy *UDPProxy) Run() {
	for i := 0; i < 2; i++ {
		proxy.connTrackLock.Lock()
		_, err := net.DialUDP("udp", nil, nil)
		if err != nil {
			/// Missing unlock here
			continue
		}
		if i == 0 {
			break
		}
	}
	proxy.connTrackLock.Unlock()
}
func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			proxy := &UDPProxy{}
			// deadlocks: x > 0
			go proxy.Run()
		}()
	}
}
