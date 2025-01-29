package main

import (
	"fmt"
	"runtime"
	"sync"
)

func init() {
	fmt.Println("Starting run...")
}

func main() {
	defer func() {
		runtime.Gosched()
		runtime.GC()
	}()

	go func() {
		// deadlocks: 0
		mu := sync.Mutex{}
		mu.Lock()
		go func() {
			// deadlocks: 0
			mu.Lock()
		}()
		mu.Unlock()
	}()
}
