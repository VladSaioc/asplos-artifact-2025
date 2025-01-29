package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

func main() {
	defer func() {
		time.Sleep(1 * time.Second)
		runtime.GC()
	}()

	for i := 0; i <= 1000; i++ {
		go func() {
			// deadlocks: 0
			c := make(chan os.Signal, 1)
			signal.Notify(c, syscall.SIGTERM)
			<-c
		}()
	}
}
