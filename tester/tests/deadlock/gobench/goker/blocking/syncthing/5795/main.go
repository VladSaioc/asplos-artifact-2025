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
	go c.readerLoop()
	go func() {
		// deadlocks: 1
		c.dispatcherLoop()
	}()
}

func (c *rawConnection) readerLoop() {
	for {
		select {
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
	c.internalClose() // FIX: go c.internalClose()
}

func NewConnection(receiver Model) Connection {
	return &rawConnection{
		dispatcherLoopStopped: make(chan struct{}),
		closed:                make(chan struct{}),
		inbox:                 make(chan message),
		receiver:              receiver,
	}
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	go func() {
		// deadlocks: 1
		m := newTestModel()
		c := NewConnection(m).(*rawConnection)
		m.ccFn = func() {
			c.Close()
		}

		c.Start()
		c.inbox <- &ClusterConfig{}

		<-c.dispatcherLoopStopped
	}()
}
