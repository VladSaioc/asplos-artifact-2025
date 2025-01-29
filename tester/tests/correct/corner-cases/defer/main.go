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
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}()

	go func() {
		defer func() {
			fmt.Println("panic: Should not run!")
		}()
		<-make(chan int)
	}()

}
