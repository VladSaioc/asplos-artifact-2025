/*
 * Project: moby
 * Issue or PR  : https://github.com/moby/moby/pull/28462
 * Buggy version: b184bdabf7a01c4b802304ac64ac133743c484be
 * fix commit-id: 89b123473774248fc3a0356dd3ce5b116cc69b29
 * Flaky: 69/100
 * Description:
 *   There are three goroutines mentioned in the bug report Moby#28405.
 * Actually, only two goroutines are needed to trigger this bug. This bug
 * is another example where lock and channel are mixed with each other.
 *
 * Moby#28405 : https://github.com/moby/moby/issues/28405
 */
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

type State struct {
	Health *Health
}

type Container struct {
	sync.Mutex
	State *State
}

func (ctr *Container) start() {
	go ctr.waitExit()
}
func (ctr *Container) waitExit() {

}

type Store struct {
	ctr *Container
}

func (s *Store) Get() *Container {
	return s.ctr
}

type Daemon struct {
	containers Store
}

func (d *Daemon) StateChanged() {
	c := d.containers.Get()
	c.Lock()
	d.updateHealthMonitorElseBranch(c)
	defer c.Unlock()
}

func (d *Daemon) updateHealthMonitorIfBranch(c *Container) {
	h := c.State.Health
	if stop := h.OpenMonitorChannel(); stop != nil {
		go monitor(c, stop)
	}
}
func (d *Daemon) updateHealthMonitorElseBranch(c *Container) {
	h := c.State.Health
	h.CloseMonitorChannel()
}

type Health struct {
	stop chan struct{}
}

func (s *Health) OpenMonitorChannel() chan struct{} {
	return s.stop
}

func (s *Health) CloseMonitorChannel() {
	if s.stop != nil {
		s.stop <- struct{}{}
	}
}

func monitor(c *Container, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
			handleProbeResult(c)
		}
	}
}

func handleProbeResult(c *Container) {
	runtime.Gosched()
	c.Lock()
	defer c.Unlock()
}

func NewDaemonAndContainer() (*Daemon, *Container) {
	c := &Container{
		State: &State{&Health{make(chan struct{})}},
	}
	d := &Daemon{Store{c}}
	return d, c
}

///
/// G1							G2
/// monitor()
/// handleProbeResult()
/// 							d.StateChanged()
/// 							c.Lock()
/// 							d.updateHealthMonitorElseBranch()
/// 							h.CloseMonitorChannel()
/// 							s.stop <- struct{}{}
/// c.Lock()
/// ----------------------G1,G2 deadlock------------------------
///

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 10000; i++ {
		go func() {
			d, c := NewDaemonAndContainer()
			// deadlocks: x > 0
			go monitor(c, c.State.Health.OpenMonitorChannel()) // G1
			// deadlocks: x > 0
			go d.StateChanged() // G2
		}()
	}
}
