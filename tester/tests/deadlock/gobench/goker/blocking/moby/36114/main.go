/*
 * Project: moby
 * Issue or PR  : https://github.com/moby/moby/pull/36114
 * Buggy version: 6d4d3c52ae7c3f910bfc7552a2a673a8338e5b9f
 * fix commit-id: a44fcd3d27c06aaa60d8d1cbce169f0d982e74b1
 * Flaky: 100/100
 * Description:
 *   This is a double lock bug. The the lock for the
 * struct svm has already been locked when calling
 * svm.hotRemoveVHDsAtStart()
 */
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

type serviceVM struct {
	sync.Mutex
}

func (svm *serviceVM) hotAddVHDsAtStart() {
	svm.Lock()
	defer svm.Unlock()
	svm.hotRemoveVHDsAtStart()
}

func (svm *serviceVM) hotRemoveVHDsAtStart() {
	svm.Lock()
	defer svm.Unlock()
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			s := &serviceVM{}
			// deadlocks: x > 0
			go s.hotAddVHDsAtStart()
		}()
	}
}
