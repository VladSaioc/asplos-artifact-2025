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

func main() {
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	go func() {
		mu := sync.RWMutex{}
		mu.RLock()
		go func() {
			// deadlocks: 1
			mu.Lock()
		}()
	}()

	go func() {
		mu := sync.RWMutex{}
		mu.Lock()
		go func() {
			// deadlocks: 1
			mu.RLock()
		}()
	}()
	runtime.Gosched()
}
