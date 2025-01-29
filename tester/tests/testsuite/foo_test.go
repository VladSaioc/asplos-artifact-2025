package foo

import (
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPartialDeadlock(t *testing.T) {
	_, err := os.ReadFile("testdata/foo")
	defer require.NoError(t, err)
	defer runtime.GC()
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	defer func() { <-time.After(1 * time.Second) }()
	defer runtime.Gosched()
	wg.Add(10)
	for i := 10; i > 0; i-- {
		go func() {
			var c chan int = make(chan int)
			wg.Done()
			<-c
		}()
	}
}
