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
	var foo [8]*func()
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 8; i++ {
		foo[i] = new(func())
		// deadlocks: 0
		go func(i int) {
			ch := make(chan int)
			*foo[i] = func() {
				<-ch
			}
			go func() {
				// deadlocks: 0
				time.Sleep(5000 * time.Millisecond)
				(*foo[i])()
			}()
			go func() {
				// deadlocks: 0
				<-ch
			}()
		}(i)
	}
}
