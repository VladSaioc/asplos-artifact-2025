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
	fmt.Println("here")

	for i := 0; i < 10; i++ {
		go func() {
			// deadlocks: x > 0
			mu := sync.Mutex{}
			mu.Lock()
			mu.Lock()
		}()
	}
}
