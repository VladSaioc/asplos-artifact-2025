/*
 * Project: etcd
 * Issue or PR  : https://github.com/etcd-io/etcd/pull/6857
 * Buggy version: 7c8f13aed7fe251e7066ed6fc1a090699c2cae0e
 * fix commit-id: 7afc490c95789c408fbc256d8e790273d331c984
 * Flaky: 19/100
 */
package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type Status struct{}

type node struct {
	status chan chan Status
	stop   chan struct{}
	done   chan struct{}
}

func (n *node) Status() Status {
	c := make(chan Status)
	select {
	case n.status <- c:
		return <-c
	case <-n.done:
		return Status{}
	}
}

func (n *node) run() {
	for {
		select {
		case c := <-n.status:
			c <- Status{}
		case <-n.stop:
			close(n.done)
			return
		}
	}
}

func (n *node) Stop() {
	select {
	case n.stop <- struct{}{}:
	case <-n.done:
		return
	}
	<-n.done
}

func NewNode() *node {
	return &node{
		status: make(chan chan Status),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

///
/// G1				G2				G3
/// n.run()
///									n.Stop()
///									n.stop<-
/// <-n.stop
///									<-n.done
/// close(n.done)
///	return
///									return
///					n.Status()
///					n.status<-
///----------------G2 leak-------------------
///

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
					n := NewNode()
					// deadlocks: 0
					go n.run() // G1
					// deadlocks: 0
					go n.Status() // G2
					// deadlocks: 0
					go n.Stop() // G3
				}()
			}
		}
	}()
}
