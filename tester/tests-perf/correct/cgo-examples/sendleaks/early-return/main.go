package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

// FixedEarlyReturn demonstrates how to avoid the leak.
// A return statement interrupts the evaluation of the parent goroutine before it can consume a message.
// However, the send operation unblocks because the channel capacity is large enough.
// Incoming error simulates an error produced internally.
func FixedEarlyReturn(err error) {
	// Create a synchronous channel
	ch := make(chan any, 1)

	go func() {
		// deadlocks: 0

		// Send something to the channel.
		// Does not deadlock, as the channel can send one message without blocking.
		ch <- struct{}{}
	}()

	if err != nil {
		// Interrupt evaluation of parent early in case of error.
		// Sender does not deadlock, because sending one item is non-blocking.
		return
	}

	// Only receive if there is no error.
	<-ch
}

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

				// deadlocks: 0
				go FixedEarlyReturn(nil)

				// deadlocks: 0
				go FixedEarlyReturn(fmt.Errorf("error"))
			}
		}
	}()
}
