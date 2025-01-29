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

type message interface{}

type ClusterConfig struct{}

type Model interface {
	ClusterConfig(message)
}

type TestModel struct {
	ccFn func()
}

func (t *TestModel) ClusterConfig(msg message) {
	if t.ccFn != nil {
		t.ccFn()
	}
}

func newTestModel() *TestModel {
	return &TestModel{}
}

type Connection interface {
	Start()
	Close()
}

type rawConnection struct {
	receiver Model

	inbox                 chan message
	dispatcherLoopStopped chan struct{}
	closed                chan struct{}
	closeOnce             sync.Once
}

func (c *rawConnection) Start() {
	// deadlocks: 0
	go c.readerLoop()
	go func() {
		// deadlocks: 0
		c.dispatcherLoop()
	}()
}

func (c *rawConnection) readerLoop() {
	for {
		select { // Orphans are detected at this control location.
		case <-c.closed:
			return
		default:
		}
	}
}

func (c *rawConnection) dispatcherLoop() {
	defer close(c.dispatcherLoopStopped)
	var msg message
	for {
		// Orphans are detected at this control location.
		// The reason is the same as in the unfixed version.
		// We need to be able to prove that the Once function is run.
		select {
		case msg = <-c.inbox:
		case <-c.closed:
			return
		}
		switch msg := msg.(type) {
		case *ClusterConfig:
			c.receiver.ClusterConfig(msg)
		default:
			return
		}
	}
}

func (c *rawConnection) internalClose() {
	c.closeOnce.Do(func() {
		close(c.closed)
		<-c.dispatcherLoopStopped
	})
}

func (c *rawConnection) Close() {
	// deadlocks: 0
	go c.internalClose()
}

func NewConnection(receiver Model) Connection {
	return &rawConnection{
		dispatcherLoopStopped: make(chan struct{}),
		closed:                make(chan struct{}),
		inbox:                 make(chan message),
		receiver:              receiver,
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

			for i := 0; i < 100; i++ {
				go func() {
					// deadlocks: 0
					m := newTestModel()
					c := NewConnection(m).(*rawConnection)
					m.ccFn = func() {
						c.Close()
					}

					c.Start()
					c.inbox <- &ClusterConfig{}

					// Due to the Once bug this blocks in one (impossible) setting
					<-c.dispatcherLoopStopped
				}()
			}
		}
	}()
}
