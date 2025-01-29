package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

func Foo() {
	// deadlocks: 1
	<-make(chan int)
}

func main() {
	defer func() {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	ch := make(chan any)
	f := func() {
		// deadlocks: 1
		<-ch
	}

	go f()
	f = Foo
	go f()

	f1 := func(_ int) {
		<-ch
	}

	// deadlocks: 1
	go f1(0)
}
