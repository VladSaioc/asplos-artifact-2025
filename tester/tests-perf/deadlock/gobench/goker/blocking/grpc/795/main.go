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

type Server struct {
	mu    sync.Mutex
	drain bool
}

func (s *Server) GracefulStop() {
	s.mu.Lock()
	if s.drain {
		s.mu.Lock()
		return
	}
	s.drain = true
	s.mu.Unlock()
}
func (s *Server) Serve() {
	s.mu.Lock()
	s.mu.Unlock()
}

func NewServer() *Server {
	return &Server{}
}

type test struct {
	srv *Server
}

func (te *test) startServer() {
	s := NewServer()
	te.srv = s
	// deadlocks: x > 0
	go s.Serve()
}

func newTest() *test {
	return &test{}
}

func testServerGracefulStopIdempotent() {
	// deadlocks: x > 0
	te := newTest()

	te.startServer()

	for i := 0; i < 3; i++ {
		te.srv.GracefulStop()
	}
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
				go testServerGracefulStopIdempotent()
			}
		}
	}()
}
