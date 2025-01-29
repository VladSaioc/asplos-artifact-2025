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
	Events       chan interface{}
	callbackChan chan func()
}

// sendEvent can be invoked many times
func (m *MultiRaft) sendEvent(event interface{}) {
	/// FIX:
	/// Let event append a event queue instead of pending here
	select {
	case m.Events <- event: // Waiting for events consumption
	case <-m.stopper.ShouldStop():
	}
}

type state struct {
	*MultiRaft
}

func (s *state) start() {
	for {
		select {
		case <-s.stopper.ShouldStop():
			return
		case cb := <-s.callbackChan:
			cb()
		default:
			s.handleWriteResponse()
			time.Sleep(time.Millisecond)
		}
	}
}

func (s *state) handleWriteResponse() {
	s.sendEvent(&EventMembershipChangeCommitted{
		Callback: func() {
			select {
			case s.callbackChan <- func() { // Waiting for callbackChan consumption
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

func (s *Store) processRaft() {
	for {
		select {
		case e := <-s.multiraft.Events:
			switch e := e.(type) {
			case *EventMembershipChangeCommitted:
				callback := e.Callback
				runtime.Gosched()
				if callback != nil {
					callback() // Waiting for callbackChan consumption
				}
			}
		case <-s.multiraft.stopper.ShouldStop():
			return
		}
	}
}

func NewStoreAndState() (*Store, *state) {
	stopper := &Stopper{
		Done: make(chan bool),
	}
	mltrft := &MultiRaft{
		stopper:      stopper,
		Events:       make(chan interface{}),
		callbackChan: make(chan func()),
	}
	st := &state{mltrft}
	s := &Store{mltrft}
	return s, st
}

func main() {
	defer func() {
		time.Sleep(time.Second)
		runtime.GC()
	}()
	for i := 0; i < 1000; i++ {
		go func() {
			s, st := NewStoreAndState()
			// deadlocks: x > 0
			go s.processRaft() // G1
			// deadlocks: x > 0
			go st.start() // G2
		}()
	}
}

// Example of deadlock trace:
//
//	G1													G2
//	--------------------------------------------------------------------------------------------------
//	s.processRaft()										st.start()
//	select												.
//	.													select [default]
//	.													s.handleWriteResponse()
//	.													s.sendEvent()
//	.													select
//	<-s.multiraft.Events <---->							m.Events <- event
//	.													select [default]
//	.													s.handleWriteResponse()
//	.													s.sendEvent()
//	.													select [m.Events<-, <-s.stopper.ShouldStop()]
//	callback()
//	select [m.callbackChan<-,<-s.stopper.ShouldStop()]	.
