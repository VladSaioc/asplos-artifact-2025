package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

func FixedNCastLeak(items []any) {
	// Do not communicate if the list is empty. Receiver does not block
	if len(items) == 0 {
		return
	}
	// The maximum payload of the channel is len(items). All senders unblock
	ch := make(chan any, len(items))

	for _, item := range items {
		go func(item any) {
			// deadlocks: 0
			ch <- struct{}{}
		}(item)
	}
	// Retrieve first result. Senders do not unblock
	// Receiver is not executed if there are no senders.
	<-ch
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	// deadlocks: 0
	go FixedNCastLeak(nil)

	// deadlocks: 0
	go FixedNCastLeak(make([]any, 100))
}
