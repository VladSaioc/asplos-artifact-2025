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

type processorListener struct {
	lock sync.RWMutex
	cond sync.Cond

	pendingNotifications []interface{}
}

func (p *processorListener) add(notification interface{}) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.pendingNotifications = append(p.pendingNotifications, notification)
	p.cond.Broadcast()
}

func (p *processorListener) pop(stopCh <-chan struct{}) {
	p.lock.Lock()
	runtime.Gosched()
	defer p.lock.Unlock()
	for {
		for len(p.pendingNotifications) == 0 {
			select {
			case <-stopCh:
				return
			default:
			}
			p.cond.Wait() //@ releases, fp
		}
		select {
		case <-stopCh: //@ blocks
			return
		}
	}
}

func newProcessListener() *processorListener {
	ret := &processorListener{
		pendingNotifications: []interface{}{},
	}
	ret.cond.L = &ret.lock
	return ret
}
func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 3000; i++ {
		go func() {
			// deadlocks: x > 0
			pl := newProcessListener()
			stopCh := make(chan struct{})
			defer close(stopCh)
			pl.add(1)
			runtime.Gosched()
			// deadlocks: x > 0
			go pl.pop(stopCh)

			resultCh := make(chan struct{})
			go func() {
				// deadlocks: x > 0
				pl.lock.Lock() //@ blocks
				close(resultCh)
			}()
			runtime.Gosched()
			<-resultCh //@ blocks
			pl.lock.Unlock()
		}()
	}
}
