/*
 * Project: grpc-go
 * Issue or PR  : https://github.com/grpc/grpc-go/pull/1275
 * Buggy version: (missing)
 * fix commit-id: 0669f3f89e0330e94bb13fa1ce8cc704aab50c9c
 * Flaky: 100/100
 * Description:
 *   Two goroutines are invovled in this deadlock. The first goroutine
 * is the main goroutine. It is blocked at case <- donec, and it is
 * waiting for the second goroutine to close the channel.
 *   The second goroutine is created by the main goroutine. It is blocked
 * when calling stream.Read(). stream.Read() invokes recvBufferRead.Read().
 * The second goroutine is blocked at case i := r.recv.get(), and it is
 * waiting for someone to send a message to this channel.
 *   It is the client.CloseSream() method called by the main goroutine that
 * should send the message, but it is not. The patch is to send out this message.
 */
package main

import (
	"fmt"
	"io"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type recvBuffer struct {
	c chan bool
}

func (b *recvBuffer) get() <-chan bool {
	return b.c
}

type recvBufferReader struct {
	recv *recvBuffer
}

func (r *recvBufferReader) Read(p []byte) (int, error) {
	select {
	// Missing data-flow due to missing modelling of io.ReadFull
	case <-r.recv.get():
	}
	return 1, nil
}

type Reader interface {
	io.Reader
	put()
}

func (r *recvBufferReader) put() {
	// Reader is not reached due to missing data-flow
	r.recv.c <- true
}

type Stream struct {
	trReader Reader
}

func (s *Stream) Read(p []byte) (int, error) {
	return io.ReadFull(s.trReader, p)
}

type http2Client struct{}

func (t *http2Client) CloseStream(s *Stream) {
	// It is the client.CloseStream() method called by the
	// main goroutine that should send the message, but it
	// is not. The patch is to send out this message.
	s.trReader.put()
}

func (t *http2Client) NewStream() *Stream {
	return &Stream{
		trReader: &recvBufferReader{
			recv: &recvBuffer{
				c: make(chan bool),
			},
		},
	}
}

func testInflightStreamClosing() {
	// deadlocks: 0
	client := &http2Client{}
	stream := client.NewStream()
	donec := make(chan bool)
	go func() { // G2
		// deadlocks: 0
		defer close(donec)
		stream.Read([]byte{1})
	}()

	// GoLive: this statement is moved up before the analysis starts to remove spurious cycle
	timeout := time.NewTimer(300 * time.Nanosecond)
	time.Sleep(20 * time.Millisecond) // GoLive: force the other goroutine to run before hitting select
	client.CloseStream(stream)

	select {
	case <-donec:
		if !timeout.Stop() {
			<-timeout.C
		}
	case <-timeout.C:
	}
}

///
/// G1 									G2
/// testInflightStreamClosing()
/// 									stream.Read()
/// 									io.ReadFull()
/// 									<- r.recv.get()
/// CloseStream()
/// <- donec
/// ------------G1 timeout, G2 leak---------------------
///

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 1; i++ {
		go testInflightStreamClosing() // G1
	}
}
