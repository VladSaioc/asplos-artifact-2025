/*
 * Project: grpc
 * Issue or PR  : https://github.com/grpc/grpc-go/pull/1353
 * Buggy version: 8264d619d80050c74e3ced8229869f525f9b877a
 * fix commit-id: 0662e89ba5974de14b442d5076627bae08071188
 * Flaky: 100/100
 * Description:
 *   When it occurs?
 *   (1) roundRobin watchAddrUpdates sends an update to gRPC,
 * and lbWatcher starts to process the update
 *   (2) roundRobin watchAddrUpdates sends another update to
 * gRPC (while holding the mutex) this send blocks because the
 * reader in lbWatcher is not reading. Also, the mutex is not
 * released until the send unblocks.
 *   (3) lbWatcher calls down when processing the previous update.
 * Since it removes some address, it tries to hold the mutex
 * and blocks
 *
 *   watchAddrUpdates is waiting for lbwatcher to read from the
 * channel, while lbwatcher is waiting for watchAddrUpdates to
 * release the mutex.
 *   The patch is to use an buffered channel and asks watchAddrUpdates
 * to drain the channel before sending message, so that watchAddrUpdates
 * will not be blocked at sending messages and it can release the lock.
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
	Start()
	Up() func()
	Notify() <-chan bool
	Close()
}

type roundRobin struct {
	mu     sync.Mutex
	addrCh chan bool
}

func (rr *roundRobin) Start() {
	rr.addrCh = make(chan bool)
	go func() { // G2
		for i := 0; i < 100; i++ {
			rr.watchAddrUpdates()
		}
	}()
}

func (rr *roundRobin) Up() func() {
	return func() {
		rr.down()
	}
}

func (rr *roundRobin) Notify() <-chan bool {
	return rr.addrCh
}

func (rr *roundRobin) Close() {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.addrCh != nil {
		close(rr.addrCh)
	}
}

func (rr *roundRobin) watchAddrUpdates() {
	rr.mu.Lock()
	runtime.Gosched()
	defer runtime.Gosched()
	defer rr.mu.Unlock()
	rr.addrCh <- true
}

func (rr *roundRobin) down() {
	rr.mu.Lock()
	defer rr.mu.Unlock()
}

type addrConn struct {
	mu   sync.Mutex
	down func()
}

func (ac *addrConn) tearDown() {
	ac.mu.Lock()
	runtime.Gosched()
	defer ac.mu.Unlock()
	if ac.down != nil {
		runtime.Gosched()
		ac.down()
	}
}

type dialOptions struct {
	balancer Balancer
}

type ClientConn struct {
	dopts dialOptions
	conns []*addrConn
}

func (cc *ClientConn) lbWatcher() {
	for range cc.dopts.balancer.Notify() {
		runtime.Gosched()
		var del []*addrConn
		for _, a := range cc.conns {
			del = append(del, a)
		}
		for _, c := range del {
			c.tearDown()
		}
	}
}

func NewClientConn() *ClientConn {
	cc := &ClientConn{
		dopts: dialOptions{
			&roundRobin{},
		},
	}
	ac1 := &addrConn{
		down: cc.dopts.balancer.Up(),
	}
	ac2 := &addrConn{
		down: cc.dopts.balancer.Up(),
	}
	cc.conns = append(cc.conns, ac1, ac2)
	return cc
}

// /
// / G1 					G2							G3
// / balancer.Start()
// / 					rr.watchAddrUpdates()
// / return
// / 												lbWatcher()
// / 												<-rr.addrCh
// / 					rr.mu.Lock()
// / 					rr.addrCh <- true
// / 					rr.mu.Unlock()
// / 												c.tearDown()
// / 												ac.down()
// / 					rr.mu.Lock()
// / 												rr.mu.Lock()
// / 					rr.addrCh <- true
// / ----------------------G2, G3 deadlock-----------------------
// /
func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	for i := 0; i < 100; i++ {
		go func() {
			cc := NewClientConn()
			cc.dopts.balancer.Start() // G1
			go cc.lbWatcher()         // G3
			go func() {
				time.Sleep(300 * time.Nanosecond)
				cc.dopts.balancer.Close()
			}()
		}()
	}
}
