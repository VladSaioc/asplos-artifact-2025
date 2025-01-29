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

	wg := &sync.WaitGroup{}
	go func() {
		// deadlocks: 0
		wg.Wait()
	}()
}
