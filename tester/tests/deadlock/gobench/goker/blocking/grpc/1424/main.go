/*
 * Project: grpc-go
 * Issue or PR  : https://github.com/grpc/grpc-go/pull/1424
 * Buggy version: 39c8c3866d926d95e11c03508bf83d00f2963f91
 * fix commit-id: 64bd0b04a7bb1982078bae6a2ab34c226125fbc1
 * Flaky: 100/100
 * Description:
 *   The parent function could return without draining the done channel.
 */
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

type Balancer interface {
	Notify() <-chan bool
}

type roundRobin struct {
	mu     sync.Mutex
	addrCh chan bool
}

func (rr *roundRobin) Notify() <-chan bool {
	return rr.addrCh
}

type addrConn struct {
	mu sync.Mutex
}

func (ac *addrConn) tearDown() {
	ac.mu.Lock()
	defer ac.mu.Unlock()
}

type dialOptions struct {
	balancer Balancer
}

type ClientConn struct {
	dopts dialOptions
	conns []*addrConn
}

func (cc *ClientConn) lbWatcher(doneChan chan bool) {
	for addr := range cc.dopts.balancer.Notify() {
		if addr {
			// nop, make compiler happy
		}
		var (
			/// add []Address is empty
			del []*addrConn
		)
		for _, a := range cc.conns {
			del = append(del, a)
		}
		for _, c := range del {
			c.tearDown()
		}
		/// FIX: defer close(doneChan)
	}
}

func NewClientConn() *ClientConn {
	cc := &ClientConn{
		dopts: dialOptions{
			&roundRobin{addrCh: make(chan bool)},
		},
	}
	return cc
}

func DialContext() {
	cc := NewClientConn()
	waitC := make(chan error, 1)
	go func() { // G2
		defer close(waitC)
		// deadlocks: 1
		ch := cc.dopts.balancer.Notify()
		if ch != nil {
			doneChan := make(chan bool)
			go cc.lbWatcher(doneChan) // G3
			<-doneChan
		}
	}()
	/// close addrCh
	close(cc.dopts.balancer.(*roundRobin).addrCh)
}

///
/// G1                      G2                          G3
/// DialContext()
///                         cc.dopts.balancer.Notify()
///                                                     cc.lbWatcher()
///                         <-doneChan
/// close()
/// -----------------------G2 leak------------------------------------
///

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	go DialContext() // G1
}
