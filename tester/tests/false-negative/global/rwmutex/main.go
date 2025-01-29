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

var mu = &sync.RWMutex{}

func main() {
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	mu.RLock()
	go func() {
		// deadlocks: 0
		mu.Lock()
	}()
}
