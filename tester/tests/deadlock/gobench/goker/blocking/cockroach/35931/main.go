package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type RowReceiver interface {
	Push()
}

type inboundStreamInfo struct {
	receiver RowReceiver
}

type RowChannel struct {
	dataChan chan struct{}
}

func (rc *RowChannel) Push() {
	// The buffer size can be either 0 or 1 when this function is entered.
	// We need context sensitivity or a path-condition on the buffer size
	// to find this bug.
	rc.dataChan <- struct{}{}
}

func (rc *RowChannel) initWithBufSizeAndNumSenders(chanBufSize int) {
	rc.dataChan = make(chan struct{}, chanBufSize)
}

type flowEntry struct {
	flow           *Flow
	inboundStreams map[int]*inboundStreamInfo
}

type flowRegistry struct {
	sync.Mutex
	flows map[int]*flowEntry
}

func (fr *flowRegistry) getEntryLocked(id int) *flowEntry {
	entry, ok := fr.flows[id]
	if !ok {
		entry = &flowEntry{}
		fr.flows[id] = entry
	}
	return entry
}

func (fr *flowRegistry) cancelPendingStreamsLocked(id int) []RowReceiver {
	entry := fr.flows[id]
	pendingReceivers := make([]RowReceiver, 0)
	for _, is := range entry.inboundStreams {
		pendingReceivers = append(pendingReceivers, is.receiver)
	}
	return pendingReceivers
}

type Flow struct {
	id             int
	flowRegistry   *flowRegistry
	inboundStreams map[int]*inboundStreamInfo
}

func (f *Flow) cancel() {
	f.flowRegistry.Lock()
	timedOutReceivers := f.flowRegistry.cancelPendingStreamsLocked(f.id)
	f.flowRegistry.Unlock()

	for _, receiver := range timedOutReceivers {
		receiver.Push()
	}
}

func (fr *flowRegistry) RegisterFlow(f *Flow, inboundStreams map[int]*inboundStreamInfo) {
	entry := fr.getEntryLocked(f.id)
	entry.flow = f
	entry.inboundStreams = inboundStreams
}

func makeFlowRegistry() *flowRegistry {
	return &flowRegistry{
		flows: make(map[int]*flowEntry),
	}
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	go func() {
		// deadlocks: 1
		fr := makeFlowRegistry()

		left := &RowChannel{}
		left.initWithBufSizeAndNumSenders(1)
		right := &RowChannel{}
		right.initWithBufSizeAndNumSenders(1)

		inboundStreams := map[int]*inboundStreamInfo{
			0: {
				receiver: left,
			},
			1: {
				receiver: right,
			},
		}

		left.Push()

		flow := &Flow{
			id:             0,
			flowRegistry:   fr,
			inboundStreams: inboundStreams,
		}

		fr.RegisterFlow(flow, inboundStreams)

		flow.cancel()
	}()
}
