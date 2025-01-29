package main

import (
	"fmt"
	_ "net/http/pprof"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

// Start the state.
func start(events chan func()) {
	callbackChan := make(chan func())
	var pendingEvents func()
	for {
		var eventsChan chan func()

		if pendingEvents != nil {
			eventsChan = events
		}

		select {
		case cb := <-callbackChan:
			// A callback is waiting to be called.
			cb()
		case eventsChan <- pendingEvents:
			// The pending events are sent to the events channel.
			pendingEvents = nil
		case <-time.After(1 * time.Millisecond):
			// Add a change membership change event to the pending events.
			pendingEvents = func() {
				// Either send a callback or stop.
				callbackChan <- func() {}
			}
		}
	}
}

func processRaft(events chan func()) {
	for e := range events {
		e() // Waiting for callbackChan consumption
		<-time.After(5 * time.Millisecond)
	}
}

func main() {
	defer func() {
		time.Sleep(1000 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			events := make(chan func())
			// deadlocks: 0
			go processRaft(events) // G1
			// deadlocks: 0
			go start(events) // G2
		}()
	}
}
