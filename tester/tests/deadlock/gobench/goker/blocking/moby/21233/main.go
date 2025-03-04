/*
 * Project: moby
 * Issue or PR  : https://github.com/moby/moby/pull/21233
 * Buggy version: cc12d2bfaae135e63b1f962ad80e6943dd995337
 * fix commit-id: 2f4aa9658408ac72a598363c6e22eadf93dbb8a7
 * Flaky:100/100
 * Description:
 *   This test was checking that it received every progress update that was
 *  produced. But delivery of these intermediate progress updates is not
 *  guaranteed. A new update can overwrite the previous one if the previous
 *  one hasn't been sent to the channel yet.
 *    The call to t.Fatalf exited the cur rent goroutine which was consuming
 *  the channel, which caused a deadlock and eventual test timeout rather
 *  than a proper failure message.
 */
package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type Progress struct{}

type Output interface {
	WriteProgress(Progress) error
}

type chanOutput chan<- Progress

type TransferManager struct {
	mu sync.Mutex
}

type Transfer struct {
	mu sync.Mutex
}

type Watcher struct {
	signalChan  chan struct{}
	releaseChan chan struct{}
	running     chan struct{}
}

func ChanOutput(progressChan chan<- Progress) Output {
	return chanOutput(progressChan)
}
func (out chanOutput) WriteProgress(p Progress) error {
	out <- p
	return nil
}
func NewTransferManager() *TransferManager {
	return &TransferManager{}
}
func NewTransfer() *Transfer {
	return &Transfer{}
}
func (t *Transfer) Release(watcher *Watcher) {
	t.mu.Lock()
	t.mu.Unlock()
	close(watcher.releaseChan)
	<-watcher.running
}
func (t *Transfer) Watch(progressOutput Output) *Watcher {
	t.mu.Lock()
	defer t.mu.Unlock()
	lastProgress := Progress{}
	w := &Watcher{
		releaseChan: make(chan struct{}),
		signalChan:  make(chan struct{}),
		running:     make(chan struct{}),
	}
	go func() { // G2
		// deadlocks: x > 0
		defer func() {
			close(w.running)
		}()
		done := false
		for {
			t.mu.Lock()
			t.mu.Unlock()
			if rand.Int31n(2) >= 1 {
				progressOutput.WriteProgress(lastProgress)
			}
			if done {
				return
			}
			select {
			case <-w.signalChan:
			case <-w.releaseChan:
				done = true
			}
		}
	}()
	return w
}
func (tm *TransferManager) Transfer(progressOutput Output) (*Transfer, *Watcher) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	t := NewTransfer()
	return t, t.Watch(progressOutput)
}

func testTransfer() { // G1
	// deadlocks: x > 0
	tm := NewTransferManager()
	progressChan := make(chan Progress)
	progressDone := make(chan struct{})
	go func() { // G3
		time.Sleep(1 * time.Millisecond)
		for p := range progressChan { /// Chan consumer
			if rand.Int31n(2) >= 1 {
				return
			}
			fmt.Println(p)
		}
		close(progressDone)
	}()
	time.Sleep(1 * time.Millisecond)
	ids := []string{"id1", "id2", "id3"}
	xrefs := make([]*Transfer, len(ids))
	watchers := make([]*Watcher, len(ids))
	for i := range ids {
		xrefs[i], watchers[i] = tm.Transfer(ChanOutput(progressChan)) /// Chan producer
		time.Sleep(2 * time.Millisecond)
	}

	for i := range xrefs {
		xrefs[i].Release(watchers[i])
	}

	close(progressChan)
	<-progressDone
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	for i := 0; i < 100; i++ {
		go testTransfer() // G1
	}
}

//	Example deadlock trace:
//
// 	G1 						G2						G3
// 	------------------------------------------------------------------------------------------------
// 	testTransfer()
// 	tm.Transfer()
// 	t.Watch()
// 	.						WriteProgress()
// 	.						ProgressChan<-
// 	.						.						<-progressChan
// 	.						.						rand.Int31n(2) >= 1
// 	.						.						return
// 	.						ProgressChan<-			.
// 	<-watcher.running
// 	----------------------G1, G2 leak--------------------------
//
