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

func FixedTimeout(ctx context.Context) {
	// One message may be sent over the channel without deadlocking.
	ch := make(chan any, 1)

	go func() {
		// deadlocks: 0

		// Sending no longer deadlocks if the context is cancelled.
		ch <- struct{}{}
	}()

	select {
	case <-ctx.Done():
	case <-ch:
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
			for i := 0; i < 10; i++ {
				go func() {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()

					for i := 0; i < 100; i++ {
						// deadlocks: 0
						go FixedTimeout(ctx)
					}
				}()
			}
		}
	}()
}
