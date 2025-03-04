package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type Checkpointer func(ctx context.Context)

type lessor struct {
	mu                 sync.RWMutex
	cp                 Checkpointer
	checkpointInterval time.Duration
}

func (le *lessor) Checkpoint() {
	le.mu.Lock()
	defer le.mu.Unlock()
}

func (le *lessor) SetCheckpointer(cp Checkpointer) {
	le.mu.Lock()
	defer le.mu.Unlock()

	le.cp = cp
}

func (le *lessor) Renew() {
	le.mu.Lock()
	unlock := func() { le.mu.Unlock() }
	defer func() { unlock() }()

	if le.cp != nil {
		le.cp(context.Background())
	}
}
func main() {
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			// deadlocks: x > 0

			le := &lessor{
				checkpointInterval: 0,
			}
			fakerCheckerpointer := func(ctx context.Context) {
				le.Checkpoint()
			}
			le.SetCheckpointer(fakerCheckerpointer)
			le.mu.Lock()
			le.mu.Unlock()
			le.Renew()
		}()
	}
}
