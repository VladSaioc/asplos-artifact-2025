package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

func Gen() {
	go func() {
		// deadlocks: 1
		<-make(chan int)
	}()
	// deadlocks: 1
	<-make(chan int)
}

func Gen0[T any]() {
	go func() {
		// deadlocks: 1
		<-make(chan int)
	}()
	<-make(chan int)
}

func Gen1[T any](_ T) {
	go func() {
		// deadlocks: 1
		<-make(chan int)
	}()
	<-make(chan int)
}

func main() {
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	go Gen()
	// deadlocks: 1
	go Gen0[any]()
	// deadlocks: 1
	go Gen1(0)
}
