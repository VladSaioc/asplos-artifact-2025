package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

// Incoming list of items and the number of workers.
func FixedNoCloseRange(list []any, workers int) {
	if workers <= 0 {
		return
	}

	// The channel can accept the require number of elements
	ch := make(chan any, len(list))

	// Create each worker (can assume workers > 0)
	for i := 0; i < workers; i++ {
		go func() {
			// deadlocks: 0

			// Each worker waits for an item and processes it.
			for item := range ch {
				// Process each item
				_ = item
			}
		}()
	}

	// Send each item to one of the workers.
	for _, item := range list {
		// Sending no longer deadlocks, even if no workers are present
		ch <- item
	}
	// Close the channel once all items are sent.
	// This allows all workers to exit their range loop and terminate
	close(ch)
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	go func() {
		// deadlocks: 0
		FixedNoCloseRange(make([]any, 1), 0)
	}()

	go func() {
		// deadlocks: 0
		FixedNoCloseRange(make([]any, 100), 10)
	}()
}
