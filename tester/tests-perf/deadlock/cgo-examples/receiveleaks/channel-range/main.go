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
func NoCloseRange(list []any, workers int) {
	ch := make(chan any)

	// Create each worker
	for i := 0; i < workers; i++ {
		go func() {
			// deadlocks: x > 0

			// Each worker waits for an item and processes it.
			for item := range ch {
				// Process each item
				_ = item
			}
		}()
	}

	// Send each item to one of the workers.
	for _, item := range list {
		// Sending can deadlock if workers == 0 or if one of the workers panics
		ch <- item
	}
	// The channel is never closed, so workers deadlock once there are no more
	// items left to process.
}

func monitor() {
	var mem = runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	fmt.Println("Final goroutine count:", runtime.NumGoroutine())
}

func main() {
	defer func() {
		time.Sleep(time.Minute / 2)
		runtime.GC()

		monitor()
	}()

	go func() {
		for {
			time.Sleep(time.Second / 2)
			for i := 0; i < 100; i++ {
				go func() {
					// deadlocks: x > 0
					NoCloseRange(make([]any, 1), 0)
				}()

				go func() {
					// deadlocks: 0
					NoCloseRange(make([]any, 100), 10)
				}()
			}
		}
	}()
}
