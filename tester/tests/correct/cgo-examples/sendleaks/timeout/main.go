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

func main() {
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for i := 0; i < 100; i++ {
		// deadlocks: 0
		go FixedTimeout(ctx)
	}
}
