package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

// DoubleSend incoming channel must send a message (incoming error simulates an error generated internally).
func DoubleSend(ch chan any, err error) {
	if err != nil {
		// In case of an error, send nil.
		ch <- nil
		// Return is missing here.
	}
	// Otherwise, continue with normal behaviour
	// This send is still executed in the error case, which may lead to deadlock.
	ch <- struct{}{}
}

func main() {
	ch := make(chan any)
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	go func() {
		// deadlocks: 0
		DoubleSend(ch, nil)
	}()
	<-ch

	go func() {
		// deadlocks: 1
		DoubleSend(ch, fmt.Errorf("error"))
	}()
	<-ch

	ch1 := make(chan any, 1)
	go func() {
		// deadlocks: 0
		DoubleSend(ch1, fmt.Errorf("error"))
	}()
	<-ch1
}
