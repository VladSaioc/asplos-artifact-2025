package main

import (
	"fmt"
	"runtime"
	// "sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

func main() {
	defer func() {
		time.Sleep(5000 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i <= 1000; i++ {
		go func() {
			// deadlocks: 0
			ticker := time.NewTicker(200 * time.Millisecond)
			go func() {
				<-ticker.C
			}()
		}()
	}
}
