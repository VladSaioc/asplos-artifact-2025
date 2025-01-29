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

var mu = &sync.Mutex{}

func main() {
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	mu.Lock()
	go func() {
		// deadlocks: 0
		mu.Lock()
	}()
}
