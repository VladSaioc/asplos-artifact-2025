/*
 * Project: cockroach
 * Issue or PR  : https://github.com/cockroachdb/cockroach/pull/6181
 * Buggy version: c0a232b5521565904b851699853bdbd0c670cf1e
 * fix commit-id: d5814e4886a776bf7789b3c51b31f5206480d184
 * Flaky: 57/100
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

type testDescriptorDB struct {
	cache *rangeDescriptorCache
}

func initTestDescriptorDB() *testDescriptorDB {
	return &testDescriptorDB{&rangeDescriptorCache{}}
}

type rangeDescriptorCache struct {
	rangeCacheMu sync.RWMutex
}

func (rdc *rangeDescriptorCache) LookupRangeDescriptor() {
	rdc.rangeCacheMu.RLock()
	runtime.Gosched()
	fmt.Println("lookup range descriptor:", rdc)
	rdc.rangeCacheMu.RUnlock()
	rdc.rangeCacheMu.Lock()
	rdc.rangeCacheMu.Unlock()
}

func (rdc *rangeDescriptorCache) String() string {
	rdc.rangeCacheMu.RLock()
	defer rdc.rangeCacheMu.RUnlock()
	return rdc.stringLocked()
}

func (rdc *rangeDescriptorCache) stringLocked() string {
	return "something here"
}

func doLookupWithToken(rc *rangeDescriptorCache) {
	rc.LookupRangeDescriptor()
}

func testRangeCacheCoalescedRquests() {
	// deadlocks: x > 0
	db := initTestDescriptorDB()
	pauseLookupResumeAndAssert := func() {
		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() { // G2,G3,...
				// deadlocks: x > 0
				doLookupWithToken(db.cache)
				wg.Done()
			}()
		}
		wg.Wait()
	}
	pauseLookupResumeAndAssert()
}

/// G1 									G2							G3					...
/// testRangeCacheCoalescedRquests()
/// initTestDescriptorDB()
/// pauseLookupResumeAndAssert()
/// return
/// 									doLookupWithToken()
///																 	doLookupWithToken()
///										rc.LookupRangeDescriptor()
///																	rc.LookupRangeDescriptor()
///										rdc.rangeCacheMu.RLock()
///										rdc.String()
///																	rdc.rangeCacheMu.RLock()
///																	fmt.Printf()
///																	rdc.rangeCacheMu.RUnlock()
///																	rdc.rangeCacheMu.Lock()
///										rdc.rangeCacheMu.RLock()
/// -------------------------------------G2,G3,... deadlock--------------------------------------

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	for i := 0; i < 100; i++ {
		go testRangeCacheCoalescedRquests() // G1
	}
}
