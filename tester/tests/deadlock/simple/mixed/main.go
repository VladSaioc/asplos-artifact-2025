package main

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
)

func init() {
	fmt.Println("Starting run...")
}
func Foo(xs []any) {
	wg := sync.WaitGroup{}

	ch := make(chan any)
	errChan := make(chan error)

	wg.Add(len(xs))
	for x := range xs {
		go func(x any) {
			// deadlocks: x > 0
			defer wg.Done()
			choice := make(chan struct{}, 1)
			select {
			case choice <- struct{}{}:
				errChan <- errors.New("error")
				return
			case choice <- struct{}{}:
			}
			ch <- x
		}(x)
	}

RESULTS:
	for i := 0; i < len(xs); i++ {
		select {
		case <-errChan:
			break RESULTS
		case <-ch:
		}
	}

	wg.Wait()
	close(ch)
	close(errChan)
}

func main() {
	defer func() {
		runtime.Gosched()
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func(i int) {
			// deadlocks: x > 0
			list := make([]any, i)
			Foo(list)
		}(i)
	}
}
