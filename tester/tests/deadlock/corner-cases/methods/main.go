package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type (
	A        struct{}
	G[T any] struct{}
)

func NoParams() {
	go func() {
		// deadlocks: 1
		<-make(chan int)
	}()
	// deadlocks: 1
	<-make(chan int)
}

func Params(_ int) {
	go func() {
		// deadlocks: 1
		<-make(chan int)
	}()
	<-make(chan int)
}

func (a A) Method() {
	go func() {
		// deadlocks: 1
		<-make(chan int)
	}()
	<-make(chan int)
}

func (a *A) PtrMethod() {
	go func() {
		// deadlocks: 1
		<-make(chan int)
	}()
	<-make(chan int)
}

func (a G[T]) Method() {
	go func() {
		// deadlocks: 1
		<-make(chan int)
	}()
	<-make(chan int)
}

func (a *G[T]) PtrMethod() {
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

	x := &A{}

	go NoParams()
	// deadlocks: 1
	go Params(0)
	// deadlocks: 1
	go x.Method()
	// deadlocks: 1
	go x.PtrMethod()

	y := &G[int]{}
	// deadlocks: 1
	go y.Method()
	// deadlocks: 1
	go y.PtrMethod()
}
