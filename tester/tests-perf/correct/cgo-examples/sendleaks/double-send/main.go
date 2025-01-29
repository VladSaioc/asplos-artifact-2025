package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

// FixedDoubleSend incoming channel must send a message (incoming error simulates an error generated internally).
func FixedDoubleSend(ch chan any, err error) {
	if err != nil {
		ch <- nil
		return // Return interrupts control flow here.
	}
	// This send is no longer executed in the error case, avoiding a potential deadlock.
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
						// deadlocks: 0
						FixedDoubleSend(ch, nil)
					}()
					<-ch

					go func() {
						// deadlocks: 0
						FixedDoubleSend(ch, fmt.Errorf("error"))
					}()
					<-ch
				}()
			}
		}
	}()
}
