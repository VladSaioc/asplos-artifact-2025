/*
 * Project: moby
 * Issue or PR  : https://github.com/moby/moby/pull/33293
 * Buggy version: 4921171587c09d0fcd8086a62a25813332f44112
 * fix commit-id:
 * Flaky: 100/100
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

func MayReturnError() error {
	if rand.Int31n(2) >= 1 {
		return errors.New("Error")
	}
	return nil
}
func containerWait() <-chan error {
	errC := make(chan error, 1)
	err := MayReturnError()
	if err != nil {
		errC <- err
		return errC
	}
	return errC
}

///
/// G1
/// containerWait()
/// errC <- err
/// ---------G1 leak---------------
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
				go func() { // G1
					// deadlocks: 0
					err := containerWait()
					if err != nil {
						return
					}
				}()
			}
		}
	}()
}
