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

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		// deadlocks: 1
		wg.Wait()
	}()
}
