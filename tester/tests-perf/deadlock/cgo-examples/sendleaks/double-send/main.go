package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

// DoubleSend incoming channel must send a message (incoming error simulates an error generated internally).
func DoubleSend(ch chan any, err error) {
	if err != nil {
		// In case of an error, send nil.
		ch <- nil
		// Return is missing here.
	}
	// Otherwise, continue with normal behaviour
	// This send is still executed in the error case, which may lead to deadlock.
	ch <- struct{}{}
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
					ch := make(chan any)
					go func() {
						// deadlocks: x > 0
						DoubleSend(ch, nil)
					}()
					<-ch

					go func() {
						// deadlocks: x > 0
						DoubleSend(ch, fmt.Errorf("error"))
					}()
					<-ch
				}()
			}
		}
	}()
}
