/*
 * Project: grpc-go
 * Issue or PR  : https://github.com/grpc/grpc-go/pull/660
 * Buggy version: db85417dd0de6cc6f583672c6175a7237e5b5dd2
 * fix commit-id: ceacfbcbc1514e4e677932fd55938ac455d182fb
 * Flaky: 100/100
 * Description:
 *   The parent function could return without draining the done channel.
 */
package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type benchmarkClient struct {
	stop chan bool
}

func (bc *benchmarkClient) doCloseLoopUnary() {
	done := make(chan bool)
	for {
		go func() { // G2
			// deadlocks: 0
			if rand.Intn(10) > 7 {
				select {
				case <-bc.stop:
				case done <- false:
				}
				return
			}

			select {
			case <-bc.stop:
			case done <- true:
			}
		}()
		select {
		case <-bc.stop:
			return
		case <-done:
		}
	}
}

///
/// G1 						G2 				helper goroutine
/// doCloseLoopUnary()
///											bc.stop <- true
/// <-bc.stop
/// return
/// 						done <-
/// ----------------------G2 leak--------------------------
///

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			// deadlocks: 0
			bc := &benchmarkClient{
				stop: make(chan bool),
			}
			// deadlocks: 0
			go bc.doCloseLoopUnary() // G1
			close(bc.stop)
		}()
	}
}
