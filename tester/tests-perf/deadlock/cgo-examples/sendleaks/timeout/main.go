package main

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

// A context is provided to short-circuit evaluation.
func Timeout(ctx context.Context) {
	ch := make(chan any)

	go func() {
		// deadlocks: x > 20
		ch <- struct{}{}
	}()

	runtime.Gosched()
	select {
	case <-ch: // Receive message
	// Sender is released
	case <-ctx.Done(): // Context was cancelled or timed out
		// Sender is stuck
	}
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

			for i := 0; i < 1; i++ {
				go func() {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()

					for i := 0; i < 100; i++ {
						go Timeout(ctx)
					}
				}()
			}
		}
	}()
}
