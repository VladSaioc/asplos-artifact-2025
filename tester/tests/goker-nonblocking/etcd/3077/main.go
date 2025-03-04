package main

import (
	"runtime"
	"time"
)

type raftNode struct {
	s       *EtcdServer
	stopped chan struct{}
	done    chan struct{}
}

func (r *raftNode) run() {
	r.stopped = make(chan struct{})
	r.done = make(chan struct{})
	defer r.stop()
	for {
		select {
		case <-r.stopped:
			return
		}
	}
}

func (r *raftNode) stop() {
	close(r.done)
}

type EtcdServer struct {
	r    raftNode
	done chan struct{}
	stop chan struct{}
}

func (s *EtcdServer) run() {
	go s.r.run()
	// Wait s.r.run
	time.Sleep(10 * time.Millisecond)
	defer func() {
		s.r.stopped <- struct{}{}
		<-s.r.done
		close(s.done)
	}()

	for {
		select {
		case <-s.stop:
			return
		}
	}
}

func (s *EtcdServer) start() {
	s.done = make(chan struct{})
	s.stop = make(chan struct{})
	go s.run()
}

func (s *EtcdServer) Stop() {
	select {
	case s.stop <- struct{}{}:
	case <-s.done:
		return
	}
	<-s.done
}

func main() {
	defer func() {
		time.Sleep(1 * time.Second)
		runtime.GC()
	}()
	for i := 0; i <= 100; i++ {
		go func() {
			srv := &EtcdServer{
				r: raftNode{},
			}
			srv.start()
			srv.Stop()
		}()
	}
}
