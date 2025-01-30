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
		// deadlocks: x > 0
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

func main() {
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for i := 0; i < 100; i++ {
		go Timeout(ctx)
	}
}
