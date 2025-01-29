/*
 * Project: grpc
 * Issue or PR  : https://github.com/grpc/grpc-go/pull/1460
 * Buggy version: 7db1564ba1229bc42919bb1f6d9c4186f3aa8678
 * fix commit-id: e605a1ecf24b634f94f4eefdab10a9ada98b70dd
 * Flaky: 100/100
 * Description:
 *   When gRPC keepalives are enabled (which isn't the case
 * by default at this time) and PermitWithoutStream is false
 * (the default), the client can deadlock when transitioning
 * between having no active stream and having one active
 * stream.The keepalive() goroutine is stuck at “<-t.awakenKeepalive”,
 * while the main goroutine is stuck in NewStream() on t.mu.Lock().
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

type Stream struct{}

type http2Client struct {
	mu              sync.Mutex
	awakenKeepalive chan struct{}
	activeStream    []*Stream
}

func (t *http2Client) keepalive() {
	t.mu.Lock()
	if len(t.activeStream) < 1 {
		<-t.awakenKeepalive
		runtime.Gosched()
		t.mu.Unlock()
	} else {
		t.mu.Unlock()
	}
}

func (t *http2Client) NewStream() {
	t.mu.Lock()
	runtime.Gosched()
	t.activeStream = append(t.activeStream, &Stream{})
	if len(t.activeStream) == 1 {
		select {
		case t.awakenKeepalive <- struct{}{}:
			// FIX: t.awakenKeepalive <- struct{}{}
		default:
		}
	}
	t.mu.Unlock()
}

///
/// G1 						G2
/// client.keepalive()
/// 						client.NewStream()
/// t.mu.Lock()
/// <-t.awakenKeepalive
/// 						t.mu.Lock()
/// ---------------G1, G2 deadlock--------------
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
					client := &http2Client{
						awakenKeepalive: make(chan struct{}),
					}
					// deadlocks: x > 0
					go client.keepalive() //G1
					// deadlocks: x > 0
					go client.NewStream() //G2
				}()
			}
		}
	}()
}
