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

	go func() {
		// deadlocks: 1
		<-make(chan int)
	}()

	time.Sleep(100 * time.Millisecond)
}
