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

type addrConn struct {
	mu    sync.Mutex // L1
	cc    *ClientConn
	addr  Address
	dopts dialOptions
	down  func()
}

func (ac *addrConn) tearDown() {
	ac.mu.Lock()         // L1
	defer ac.mu.Unlock() // L1
	if ac.down != nil {
		ac.down()
		ac.down = nil
	}
}

func (ac *addrConn) resetTransport() {
	ac.mu.Lock() // L1
	if ac.cc.dopts.balancer != nil {
		ac.down = ac.cc.dopts.balancer.Up(ac.addr)
	}
	ac.mu.Unlock() // L1
}

type ClientConn struct {
	dopts dialOptions
	rwmu  sync.RWMutex // L2
	conns map[Address]*addrConn
}

func (cc *ClientConn) lbWatcher() {
	for addrs := range cc.dopts.balancer.Notify() {
		var (
			add []Address
			del []*addrConn
		)
		cc.rwmu.Lock() // L2
		for _, a := range addrs {
			if _, ok := cc.conns[a]; !ok {
				add = append(add, a)
			}
		}

		for k, c := range cc.conns {
			var keep bool
			for _, a := range addrs {
				if k == a {
					keep = true
					break
				}
			}
			if !keep {
				del = append(del, c)
				delete(cc.conns, c.addr)
			}
		}
		cc.rwmu.Unlock() // L2
		for _, a := range add {
			cc.resetAddrConn(a)
		}
		for _, c := range del {
			c.tearDown()
		}
	}
}

func (cc *ClientConn) resetAddrConn(addr Address) {
	ac := &addrConn{
		cc:    cc,
		addr:  addr,
		dopts: cc.dopts,
	}
	cc.rwmu.Lock() // L2
	if cc.conns == nil {
		cc.rwmu.Unlock() // L2
		return
	}
	cc.conns[ac.addr] = ac
	cc.rwmu.Unlock() // L2
	// deadlocks: x > 0
	go ac.resetTransport()
}

func (cc *ClientConn) Close() {
	cc.rwmu.Lock() // L2
	conns := cc.conns
	cc.conns = nil
	cc.rwmu.Unlock() // L2
	if cc.dopts.balancer != nil {
		cc.dopts.balancer.Close()
	}
	for _, ac := range conns {
		ac.tearDown()
	}
}

type dialOptions struct {
	balancer Balancer
}
type DialOption func(*dialOptions)

func Dial(opts ...DialOption) *ClientConn {
	return DialContext(context.Background(), opts...)
}

func DialContext(ctx context.Context, opts ...DialOption) *ClientConn {
	cc := &ClientConn{
		conns: make(map[Address]*addrConn),
	}
	for _, opt := range opts {
		opt(&cc.dopts)
	}
	// deadlocks: x > 0
	go cc.lbWatcher() // G2
	return cc
}

type Balancer interface {
	Up(addr Address) (down func())
	Notify() <-chan []Address
	Close()
}

type Address int

type simpleBalancer struct {
	addrs    []Address
	notifyCh chan []Address
	rwmu     sync.RWMutex // L3
	closed   bool
	pinAddr  Address
}

func (b *simpleBalancer) Up(addr Address) func() {
	b.rwmu.Lock()         // L3
	defer b.rwmu.Unlock() // L3

	if b.closed {
		return func() {}
	}

	if b.pinAddr == 0 {
		b.pinAddr = addr
		b.notifyCh <- []Address{addr}
	}

	return func() {
		b.rwmu.Lock() // L3
		runtime.Gosched()
		if b.pinAddr == addr {
			b.pinAddr = 0
			b.notifyCh <- b.addrs
		}
		b.rwmu.Unlock() // L3
	}
}

func (b *simpleBalancer) Notify() <-chan []Address {
	return b.notifyCh
}

func (b *simpleBalancer) Close() {
	b.rwmu.Lock()         // L3
	defer b.rwmu.Unlock() // L3
	if b.closed {
		return
	}
	b.closed = true
	close(b.notifyCh)
	b.pinAddr = 0
}

func newSimpleBalancer() *simpleBalancer {
	notifyCh := make(chan []Address, 1)
	addrs := make([]Address, 3)
	for i := 0; i < 3; i++ {
		addrs[i] = Address(i)
	}
	notifyCh <- addrs
	return &simpleBalancer{
		addrs:    addrs,
		notifyCh: notifyCh,
	}
}

func WithBalancer(b Balancer) DialOption {
	return func(o *dialOptions) {
		o.balancer = b
	}
}

func main() {
	defer func() {
		time.Sleep(1 * time.Second)
		runtime.GC()
	}()
	for i := 0; i <= 1000; i++ {
		// G1
		go func() {
			// deadlocks: x > 0
			sb := newSimpleBalancer()
			conn := Dial(WithBalancer(sb))

			closec := make(chan struct{})
			go func() { // G3
				// deadlocks: x > 0
				defer close(closec)
				sb.Close()
			}()
			// deadlocks: x > 0
			go conn.Close() // G4
			<-closec
		}()
	}
}

// 	Example deadlock trace:
//
// 	G1								G2										G3								G4
// 	----------------------------------------------------------------------------------------------------------------------------------------
//	notifyCh<- [0,1,2] [1]
//	Dial()
//	go cc.lbWatcher() [G2]
//	go func() [G3]					.
//	go conn.Close() [G4]			.										.
//	<-closec						.										.								.
// 	.								addrs := <-cc.dobts.balancer.Notify()	.								.
//	.								cc.rwmu.Lock()	[L2]					.								.
//	.								.										sb.Close()						.
//	.								.										b.rwmu.Close()					.
//	.								.										.								cc.rwmu.Lock() [L2]
//	.								cc.rwmu.Unlock() [L2]					.								.
//	.								.										b.rwmu.Lock() [L3]				.
//	FIXME: finish this trace
