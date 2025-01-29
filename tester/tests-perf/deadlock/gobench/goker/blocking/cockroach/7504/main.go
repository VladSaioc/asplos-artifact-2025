/*
 * Project: cockroach
 * Issue or PR  : https://github.com/cockroachdb/cockroach/pull/7504
 * Buggy version: bc963b438cdc3e0ad058a5282358e5aee0595e17
 * fix commit-id: cab761b9f5ee5dee1448bc5d6b1d9f5a0ff0bad5
 * Flaky: 1/100
 * Description: There are locking leaseState, tableNameCache in Release(), but
 * tableNameCache,LeaseState in AcquireByName.  It is AB and BA deadlock.
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

func MakeCacheKey(lease *LeaseState) int {
	return lease.id
}

type LeaseState struct {
	mu sync.Mutex // L1
	id int
}
type LeaseSet struct {
	data []*LeaseState
}

func (l *LeaseSet) find(id int) *LeaseState {
	return l.data[id]
}

func (l *LeaseSet) remove(s *LeaseState) {
	for i := 0; i < len(l.data); i++ {
		if s == l.data[i] {
			l.data = append(l.data[:i], l.data[i+1:]...)
			break
		}
	}
}

type tableState struct {
	tableNameCache *tableNameCache
	mu             sync.Mutex // L3
	active         *LeaseSet
}

func (t *tableState) release(lease *LeaseState) {
	t.mu.Lock()         // L3
	defer t.mu.Unlock() // L3

	s := t.active.find(MakeCacheKey(lease))
	s.mu.Lock() // L1
	runtime.Gosched()
	defer s.mu.Unlock() // L1

	t.removeLease(s)
}
func (t *tableState) removeLease(lease *LeaseState) {
	t.active.remove(lease)
	t.tableNameCache.remove(lease) // L1 acquire/release
}

type tableNameCache struct {
	mu     sync.Mutex // L2
	tables map[int]*LeaseState
}

func (c *tableNameCache) get(id int) {
	c.mu.Lock()         // L2
	defer c.mu.Unlock() // L2
	lease, ok := c.tables[id]
	if !ok {
		return
	}
	if lease == nil {
		panic("nil lease in name cache")
	}
	lease.mu.Lock()         // L1
	defer lease.mu.Unlock() // L1
}

func (c *tableNameCache) remove(lease *LeaseState) {
	c.mu.Lock() // L2
	runtime.Gosched()
	defer c.mu.Unlock() // L2
	key := MakeCacheKey(lease)
	existing, ok := c.tables[key]
	if !ok {
		return
	}
	if existing == lease {
		delete(c.tables, key)
	}
}

type LeaseManager struct {
	_          [64]byte
	tableNames *tableNameCache
	tables     map[int]*tableState
}

func (m *LeaseManager) AcquireByName(id int) {
	m.tableNames.get(id)
}

func (m *LeaseManager) findTableState(lease *LeaseState) *tableState {
	existing, ok := m.tables[lease.id]
	if !ok {
		return nil
	}
	return existing
}

func (m *LeaseManager) Release(lease *LeaseState) {
	t := m.findTableState(lease)
	t.release(lease)
}
func NewLeaseManager(tname *tableNameCache, ts *tableState) *LeaseManager {
	mgr := &LeaseManager{
		tableNames: tname,
		tables:     make(map[int]*tableState),
	}
	mgr.tables[0] = ts
	return mgr
}
func NewLeaseSet(n int) *LeaseSet {
	lset := &LeaseSet{}
	for i := 0; i < n; i++ {
		lease := new(LeaseState)
		lset.data = append(lset.data, lease)
	}
	return lset
}

func monitor() {
	var mem = runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	fmt.Println("Final goroutine count:", runtime.NumGoroutine())
}

func main() {
	defer func() {
		time.Sleep(time.Minute / 2)
		runtime.GC()

		monitor()
	}()

	go func() {
		for {
			time.Sleep(time.Second / 2)

			for i := 0; i < 100; i++ {
				go func() {
					leaseNum := 2
					lset := NewLeaseSet(leaseNum)

					nc := &tableNameCache{
						tables: make(map[int]*LeaseState),
					}
					for i := 0; i < leaseNum; i++ {
						nc.tables[i] = lset.find(i)
					}

					ts := &tableState{
						tableNameCache: nc,
						active:         lset,
					}

					mgr := NewLeaseManager(nc, ts)

					// G1
					go func() {
						// deadlocks: x > 0
						// lock L2-L1
						mgr.AcquireByName(0)
					}()

					// G2
					go func() {
						// deadlocks: x > 0
						// lock L1-L2
						mgr.Release(lset.find(0))
					}()
				}()
			}
		}
	}()
}

// Example deadlock trace:
//
//	G1								G2
//	------------------------------------------------------------------------------------------------
//	mgr.AcquireByName(0)				mgr.Release(lset.find(0))
//	m.tableNames.get(id)				.
//	c.mu.Lock() [L2]					.
//	.								t.release(lease)
//	.								t.mu.Lock() [L3]
//	.								s.mu.Lock() [L1]
//	lease.mu.Lock() [L1]				.
//	.								t.removeLease(s)
//	.								t.tableNameCache.remove(lease)
//	.								c.mu.Lock() [L2]
//	---------------------------------------G1, G2 leak----------------------------------------------
