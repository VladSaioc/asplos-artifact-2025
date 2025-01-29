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
		time.After(5 * time.Second)
		runtime.GC()
	}()

	for i := 0; i <= 10; i++ {
		go func() {
			const NUM = 100000
			wg := sync.WaitGroup{}
			wg.Add(NUM)
			for i := 0; i < NUM; i++ {
				go func() {
					chs := make(chan struct{})
					go func() {
						// deadlocks: 0
						wg.Done()
						chs <- struct{}{}
					}()
					wg.Wait()
					go func() {
						// deadlocks: 0
						<-chs
					}()
				}()
			}
		}()
	}
}
