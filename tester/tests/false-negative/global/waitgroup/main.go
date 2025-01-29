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

var wg = &sync.WaitGroup{}

func main() {
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	wg.Add(1)
	go func() {
		// deadlocks: 0
		wg.Wait()
	}()
}
