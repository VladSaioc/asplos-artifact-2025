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
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	go func() {
		var cond = sync.NewCond(&sync.Mutex{})
		// deadlocks: 1
		cond.L.Lock()
		cond.Wait()
	}()
}
