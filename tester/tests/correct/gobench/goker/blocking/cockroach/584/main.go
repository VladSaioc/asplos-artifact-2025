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

type Gossip struct {
	mu     sync.Mutex
	closed bool
}

func (g *Gossip) bootstrap() {
	for {
		g.mu.Lock()
		if g.closed {
			g.mu.Unlock()
			break
		}
		g.mu.Unlock()
	}
}

func (g *Gossip) manage() {
	for {
		g.mu.Lock()
		if g.closed {
			g.mu.Unlock()
			break
		}
		g.mu.Unlock()
	}
}
func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			// deadlocks: 0
			g := &Gossip{
				closed: true,
			}
			go func() {
				// deadlocks: 0
				g.bootstrap()
				g.manage()
			}()
		}()
	}
}
