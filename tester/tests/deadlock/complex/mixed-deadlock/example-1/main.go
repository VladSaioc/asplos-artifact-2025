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

// sleep and then quit
func doStuff(wg *sync.WaitGroup) {
	defer wg.Done()
}

// send total num of ints to ch, call GC, and then quit
func genNumbers(total int, ch chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()

	for i := 1; i <= total; i++ {
		ch <- i
	}
}

// receive from ch until it's closed (but ch is not closed ever)
func printNumbers(ch <-chan int, wg *sync.WaitGroup) {
	defer wg.Done()

	for range ch {
	}
}

// print never returns so stuck on Wait
func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	for i := 0; i < 100; i++ {
		go func() {
			// deadlocks: x > 0
			var wg sync.WaitGroup
			var numberChan = make(chan int)

			wg.Add(4)
			go doStuff(&wg)
			// deadlocks: x > 0
			go printNumbers(numberChan, &wg)
			genNumbers(3, numberChan, &wg)

			fmt.Println("Waiting for goroutines to finish.", wg)
			wg.Wait() // stuck here

			fmt.Println("Done!")
		}()
	}
}
