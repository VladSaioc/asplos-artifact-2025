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

func main() {
	defer func() {
		time.Sleep(5 * time.Second)
		runtime.GC()
	}()

	wg := sync.WaitGroup{}
	wg.Add(100000)
	for i := 0; i < 100000; i++ {
		go func() {
			ch := make(chan struct{}, 1)
			go func() {
				wg.Done()
				<-ch
			}()
			go func() {
				wg.Wait()
				ch <- struct{}{}
			}()
		}()
	}
}
