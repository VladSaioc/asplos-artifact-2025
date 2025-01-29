package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type WaitFunc func(done <-chan struct{}) <-chan struct{}

type ConditionFunc func() (done bool, err error)

func WaitFor(wait WaitFunc, fn ConditionFunc, done <-chan struct{}) error {
	c := wait(done)
	for {
		_, open := <-c
		ok, err := fn()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if !open {
			break
		}
	}
	return nil
}

func poller(interval, timeout time.Duration) WaitFunc {
	return WaitFunc(func(done <-chan struct{}) <-chan struct{} {
		ch := make(chan struct{})
		go func() {
			defer close(ch)

			tick := time.NewTicker(interval)
			defer tick.Stop()

			var after <-chan time.Time
			if timeout != 0 {
				timer := time.NewTimer(timeout)
				after = timer.C
				defer timer.Stop()
			}
			for {
				select {
				case <-tick.C:
					select {
					case ch <- struct{}{}:
					default:
					}
				case <-after:
					return
				case <-done:
					return
				}
			}
		}()

		return ch
	})
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
					stopCh := make(chan struct{})
					defer close(stopCh)
					waitFunc := poller(time.Millisecond, 80*time.Millisecond)
					var doneCh <-chan struct{}

					WaitFor(func(done <-chan struct{}) <-chan struct{} {
						doneCh = done
						return waitFunc(done)
					}, func() (bool, error) {
						time.Sleep(10 * time.Millisecond)
						return true, nil
					}, stopCh)

					<-doneCh // block here
				}()
			}
		}
	}()
}
