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

func Spawn(i int) {
	if i == 0 {
		return
	}
	wg := &sync.WaitGroup{}
	wg.Add(i + 1)
	go func() {
		wg.Done()
		// deadlocks: x > 0
		<-make(chan int)
	}()
	for j := 0; j < i; j++ {
		go func() {
			wg.Done()
			Spawn(i - 1)
		}()
	}
	wg.Wait()
	runtime.Gosched()
	runtime.GC()
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	Spawn(5)
}
