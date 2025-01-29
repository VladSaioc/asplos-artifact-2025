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
	probeInterval := 50 * time.Nanosecond
	probeTimeout := 50 * time.Nanosecond
	for {
		select {
		case <-stop:
			return
		case <-time.After(probeInterval):
			results := make(chan bool, 1)
			ctx, cancelProbe := context.WithTimeout(context.Background(), probeTimeout)
			go func() { // G3
				// deadlocks: 0
				results <- true
				close(results)
			}()
			select {
			case <-stop:
				cancelProbe()
				<-results
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
		go func() {
			// deadlocks: 0
			stop := make(chan bool)
			// deadlocks: 0
			go monitor(stop) // G1
			go func() {      // G2
				// deadlocks: 0
				time.Sleep(50 * time.Nanosecond)
				stop <- true
			}()
		}()
	}
}
