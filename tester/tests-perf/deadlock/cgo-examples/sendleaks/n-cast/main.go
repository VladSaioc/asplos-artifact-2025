package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

// Process a number of items. First result to pass the post is retrieved from the channel queue.
func NCastLeak(items []any) {
	// Channel is synchronous.
	ch := make(chan any)

	// Iterate over every item
	for range items {
		go func() {
			// deadlocks: x > 0

			// Process item and send result to channel
			ch <- struct{}{}
			// Channel is synchronous: only one sender will synchronise
		}()
	}
	// Retrieve first result. All other senders block.
	// Receiver blocks if there are no senders.
	<-ch
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
			ch <- struct{}{}
		}(item)
	}
	// Retrieve first result. Senders do not unblock
	// Receiver is not executed if there are no senders.
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
				go func() {
					// deadlocks: x > 0
					NCastLeak(nil)
				}()

				go func() {
					// deadlocks: 0
					NCastLeak(make([]any, 100))
				}()
			}
		}
	}()
}
