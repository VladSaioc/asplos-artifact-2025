package main

import (
	"fmt"
	"runtime"
	// "time"
)

func init() {
	fmt.Println("Starting run...")
}

func main() {
	defer func() {
		runtime.GC()
		runtime.GC()
		runtime.GC()
		runtime.GC()
	}()

	go func() {
		ch := make(chan int)
		for i := 0; i < 3000000; i++ {
			go func() {
				// deadlocks: 0
				<-ch
			}()
		}
		runtime.Gosched()
		close(ch)
	}()
	runtime.Gosched()
	runtime.Gosched()
}
