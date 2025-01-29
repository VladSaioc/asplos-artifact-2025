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

func main() {
	wg := sync.WaitGroup{}

	for i := 0; i <= 1000; i++ {
		go func() {
			// deadlocks: 0
			<-time.After(5000 * time.Millisecond)
		}()
	}

	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(i time.Duration) {
			<-time.After(i * 100 * time.Millisecond)
			runtime.GC()
			wg.Done()
		}(time.Duration(i))
	}
	wg.Wait()
	runtime.GC()
}
