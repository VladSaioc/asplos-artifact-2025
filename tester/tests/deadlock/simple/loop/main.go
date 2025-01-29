package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

func main() {
	defer runtime.GC()

	for i := 0; i < 100; i++ {
		go func() {
			// deadlocks: 100
			<-make(chan int)
		}()
	}

	time.Sleep(100 * time.Millisecond)
}
