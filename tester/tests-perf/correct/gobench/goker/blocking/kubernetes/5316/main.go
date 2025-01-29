/*
 * Project: kubernetes
 * Issue or PR  : https://github.com/kubernetes/kubernetes/pull/5316
 * Buggy version: c868b0bbf09128960bc7c4ada1a77347a464d876
 * fix commit-id: cc3a433a7abc89d2f766d4c87eaae9448e3dc091
 * Flaky: 100/100
 * Description:
 *   If the main goroutine selects a case that doesn’t consumes
 * the channels, the anonymous goroutine will be blocked on sending
 * to channel.
 */

package main

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

func finishRequest(timeout time.Duration, fn func() error) {
	ch := make(chan bool, 1)
	errCh := make(chan error, 1)
	go func() { // G2
		// deadlocks: 0
		if err := fn(); err != nil {
			errCh <- err
		} else {
			ch <- true
		}
	}()

	// time.Sleep(3 * time.Millisecond)
	select {
	case <-ch:
	case <-errCh:
	case <-time.After(timeout):
	}
}

///
/// G1 						G2
/// finishRequest()
/// 						fn()
/// time.After()
/// 						errCh<-/ch<-
/// --------------G2 leak----------------
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
					fn := func() error {
						time.Sleep(2 * time.Millisecond)
						if rand.Intn(10) > 5 {
							return errors.New("Error")
						}
						return nil
					}
					finishRequest(time.Millisecond, fn) // G1
				}()
			}
		}
	}()
}
