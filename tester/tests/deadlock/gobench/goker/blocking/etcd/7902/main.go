/*
 * Project: etcd
 * Issue or PR  : https://github.com/coreos/etcd/pull/7902
 * Buggy version: dfdaf082c51ba14861267f632f6af795a27eb4ef
 * fix commit-id: 87d99fe0387ee1df1cf1811d88d37331939ef4ae
 * Flaky: 100/100
 * Description:
 *   At least two goroutines are needed to trigger this bug,
 * one is leader and the other is follower. Both the leader
 * and the follower execute the code above. If the follower
 * acquires mu.Lock() firstly and enter rc.release(), it will
 * be blocked at <- rcNextc (nextc). Only the leader can execute
 * close(nextc) to unblock the follower inside rc.release().
 * However, in order to invoke rc.release(), the leader needs
 * to acquires mu.Lock().
 *   The fix is to remove the lock and unlock around rc.release().
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

type roundClient struct {
	progress int
	acquire  func()
	validate func()
	release  func()
}

func runElectionFunc() {
	// deadlocks: x > 0
	rcs := make([]roundClient, 3)
	nextc := make(chan bool)
	for i := range rcs {
		var rcNextc chan bool
		setRcNextc := func() {
			rcNextc = nextc
		}
		rcs[i].acquire = func() {}
		rcs[i].validate = func() {
			setRcNextc()
		}
		rcs[i].release = func() {
			if i == 0 { // Assume the first roundClient is the leader
				close(nextc)
				nextc = make(chan bool)
			}
			<-rcNextc // Follower is blocking here
		}
	}
	doRounds(rcs, 100)
}
func doRounds(rcs []roundClient, rounds int) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(rcs))
	for i := range rcs {
		go func(rc *roundClient) { // G2,G3
			// deadlocks: x > 0
			defer wg.Done()
			for rc.progress < rounds || rounds <= 0 {
				rc.acquire()
				mu.Lock()
				rc.validate()
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				rc.progress++
				mu.Lock()
				rc.release()
				mu.Unlock()
			}
		}(&rcs[i])
	}
	wg.Wait()
}

///
/// G1						G2 (leader)					G3 (follower)
/// runElectionFunc()
/// doRounds()
/// wg.Wait()
/// 						...
/// 						mu.Lock()
/// 						rc.validate()
/// 						rcNextc = nextc
/// 						mu.Unlock()					...
/// 													mu.Lock()
/// 													rc.validate()
/// 													mu.Unlock()
/// 													mu.Lock()
/// 													rc.release()
/// 													<-rcNextc
/// 						mu.Lock()
/// -------------------------G1,G2,G3 deadlock--------------------------
///

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	for i := 0; i < 100; i++ {
		go runElectionFunc() // G1
	}
}
