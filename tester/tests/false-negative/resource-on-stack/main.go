package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

func foo(ch chan any) {
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	go func() {
		// deadlocks: 0
		<-ch
	}()
}

func main() {
	ch := make(chan any)
	foo(ch)

	go func() {
		for {
			_ = ch
		}
	}()
}
