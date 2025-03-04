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

type Event int
type watchCacheEvent int

type cacheWatcher struct {
	sync.Mutex
	input   chan watchCacheEvent
	result  chan Event
	done    chan struct{}
	stopped bool
}

func (c *cacheWatcher) process(initEvents []watchCacheEvent) {
	for _, event := range initEvents {
		c.sendWatchCacheEvent(&event)
	}
	defer close(c.result)
	defer c.Stop()
	for {
		_, ok := <-c.input
		if !ok {
			return
		}
	}
}

func (c *cacheWatcher) sendWatchCacheEvent(event *watchCacheEvent) {
	select {
	case c.result <- Event(*event):
	case <-c.done:
	}
}

func (c *cacheWatcher) Stop() {
	c.stop()
}

func (c *cacheWatcher) stop() {
	c.Lock()
	defer c.Unlock()
	if !c.stopped {
		c.stopped = true
		close(c.done)
		close(c.input)
	}
}

func newCacheWatcher(chanSize int, initEvents []watchCacheEvent) *cacheWatcher {
	watcher := &cacheWatcher{
		input:   make(chan watchCacheEvent, chanSize),
		result:  make(chan Event, chanSize),
		done:    make(chan struct{}),
		stopped: false,
	}
	// deadlocks: 0
	go watcher.process(initEvents)
	return watcher
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			// deadlocks: 0
			initEvents := []watchCacheEvent{1, 2}
			w := newCacheWatcher(0, initEvents)
			w.Stop()
		}()
	}
}
