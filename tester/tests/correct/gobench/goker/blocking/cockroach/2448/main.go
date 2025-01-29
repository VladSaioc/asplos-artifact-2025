package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type Stopper struct {
	Done chan bool
}

func (s *Stopper) ShouldStop() <-chan bool {
	return s.Done
}

type EventMembershipChangeCommitted struct {
	Callback func()
}
type MultiRaft struct {
	stopper      *Stopper
	Events       chan []interface{}
	callbackChan chan func()
}

// sendEvent can be invoked many times
func (s *state) sendEvent(event interface{}) {
	// Append incoming event to set of pending events
}

type state struct {
	*MultiRaft

	pendingEvents []interface{}
}

// Start the state.
func (s *state) start() {
	for {
		var eventsChan chan []interface{}

		if len(s.pendingEvents) > 0 {
			eventsChan = s.Events
		}

		select {
		// case <-s.stopper.ShouldStop():
		// 	// If the stopper has been triggered, stop.
		// 	return
		case cb := <-s.callbackChan:
			// A callback is waiting to be called.
			cb()
		case eventsChan <- s.pendingEvents:
			// The pending events are sent to the events channel.
			s.pendingEvents = nil
		default:
			// If no events are pending, process the committed entry.
			s.processCommittedEntry()
			time.Sleep(time.Millisecond)
		}
	}
}

// Process the committed entry.
func (s *state) processCommittedEntry() {
	// Add a change membership change event to the pending events.
	s.sendEvent(&EventMembershipChangeCommitted{
		Callback: func() {
			// Either send a callback or stop.
			select {
			case s.MultiRaft.callbackChan <- func() { // Waiting for callbackChan consumption
				time.Sleep(time.Nanosecond)
			}:
			case <-s.stopper.ShouldStop():
			}
		},
	})
}

type Store struct {
	multiraft *MultiRaft
}

// processRaft continuously processes raft events.
func (s *Store) processRaft() {
	// Contains references to channels:
	// 	s.multiraft.Events
	// 	s.multiraft.stopper.Done
	// 	s.multiraft.callbackChan
	for {
		// Either consume a raft event or stop.
		select {
		case events := <-s.multiraft.Events:
			// Process the raft events.
			for _, e := range events {
				var callback func()
				switch e := e.(type) {
				case *EventMembershipChangeCommitted:
					// Call the callback in the event.
					callback = e.Callback
					if callback != nil {
						callback() // Waiting for callbackChan consumption
					}
				}
			}
		case <-s.multiraft.stopper.ShouldStop():
			// Stop processing raft events.
			return
		}
	}
}

func NewStoreAndState() (*Store, *state) {
	// Make a stopper with a channel
	stopper := &Stopper{
		Done: make(chan bool),
	}
	// Make a multiraft with the stopper and Events and callbackChan channels
	mltrft := &MultiRaft{
		stopper:      stopper,
		Events:       make(chan []interface{}),
		callbackChan: make(chan func()),
	}
	// Make a state (references the multiraft)
	st := &state{mltrft, []interface{}{}}
	// Make a store (references the multiraft)
	s := &Store{mltrft}
	return s, st
}

func main() {
	defer func() {
		time.Sleep(1 * time.Second)
		runtime.GC()
	}()

	for i := 0; i < 1000; i++ {
		go func() {
			// deadlocks: 0
			s, st := NewStoreAndState()
			// deadlocks: 0
			go s.processRaft() // G1
			// deadlocks: 0
			go st.start() // G2
		}()
	}
}
