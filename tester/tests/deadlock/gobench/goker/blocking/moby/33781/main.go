/*
 * Project: moby
 * Issue or PR  : https://github.com/moby/moby/pull/33781
 * Buggy version: 33fd3817b0f5ca4b87f0a75c2bd583b4425d392b
 * fix commit-id: 67297ba0051d39be544009ba76abea14bc0be8a4
 * Flaky: 25/100
 * Description:
 *   The goroutine created using anonymous function is blocked at
 * sending message to a unbuffered channel. However there exists a
 * path in the parent goroutine where the parent function will
 * return without draining the channel.
 */

package main

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

func monitor(stop chan bool) {
	probeInterval := time.Millisecond
	probeTimeout := time.Millisecond
	for {
		select {
		case <-stop:
			return
		case <-time.After(probeInterval):
			results := make(chan bool)
			ctx, cancelProbe := context.WithTimeout(context.Background(), probeTimeout)
			go func() { // G3
				// deadlocks: x > 0
				results <- true
				close(results)
			}()
			select {
			case <-stop:
				// results should be drained here
				cancelProbe()
				return
			case <-results:
				cancelProbe()
			case <-ctx.Done():
				cancelProbe()
				<-results
			}
		}
	}
}

///
/// G1 				G2				G3
/// monitor()
/// <-time.After()
/// 				stop <-
/// <-stop
/// 				return
/// cancelProbe()
/// return
/// 								result<-
///----------------G3 leak------------------
///

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	for i := 0; i < 100; i++ {
		go func(i int) {
			stop := make(chan bool)
			go monitor(stop) // G1
			go func() {      // G2
				time.Sleep(time.Duration(i) * time.Millisecond)
				stop <- true
			}()
		}(i)
	}
}
