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

	var cond = sync.NewCond(&sync.Mutex{})
	go func() {
		// deadlocks: 0
		cond.L.Lock()
		cond.Wait()
		cond.L.Unlock()
	}()
	runtime.Gosched()
	cond.Signal()
}
