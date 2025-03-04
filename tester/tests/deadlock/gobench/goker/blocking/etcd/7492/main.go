/*
 * Project: etcd
 * Issue or PR  : https://github.com/etcd-io/etcd/pull/7492
 * Buggy version: 51939650057d602bb5ab090633138fffe36854dc
 * fix commit-id: 1b1fabef8ffec606909f01c3983300fff539f214
 * Flaky: 40/100
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

type TokenProvider interface {
	assign()
	enable()
	disable()
}

type simpleTokenTTLKeeper struct {
	tokens           map[string]time.Time
	addSimpleTokenCh chan struct{}
	stopCh           chan chan struct{}
	deleteTokenFunc  func(string)
}

type authStore struct {
	tokenProvider TokenProvider
}

func (as *authStore) Authenticate() {
	as.tokenProvider.assign()
}

func NewSimpleTokenTTLKeeper(deletefunc func(string)) *simpleTokenTTLKeeper {
	stk := &simpleTokenTTLKeeper{
		tokens:           make(map[string]time.Time),
		addSimpleTokenCh: make(chan struct{}, 1),
		stopCh:           make(chan chan struct{}),
		deleteTokenFunc:  deletefunc,
	}
	// deadlocks: x > 0
	go stk.run() // G1
	return stk
}

func (tm *simpleTokenTTLKeeper) run() {
	tokenTicker := time.NewTicker(time.Nanosecond)
	defer tokenTicker.Stop()
	for {
		select {
		case <-tm.addSimpleTokenCh:
			runtime.Gosched()
			/// Make tm.tokens not empty is enough
			tm.tokens["1"] = time.Now()
		case <-tokenTicker.C:
			runtime.Gosched()
			for t, _ := range tm.tokens {
				tm.deleteTokenFunc(t)
				delete(tm.tokens, t)
			}
		case waitCh := <-tm.stopCh:
			waitCh <- struct{}{}
			return
		}
	}
}

func (tm *simpleTokenTTLKeeper) addSimpleToken() {
	tm.addSimpleTokenCh <- struct{}{}
	runtime.Gosched()
}

func (tm *simpleTokenTTLKeeper) stop() {
	waitCh := make(chan struct{})
	tm.stopCh <- waitCh
	<-waitCh
	close(tm.stopCh)
}

type tokenSimple struct {
	simpleTokenKeeper *simpleTokenTTLKeeper
	simpleTokensMu    sync.RWMutex
}

func (t *tokenSimple) assign() {
	t.assignSimpleTokenToUser()
}

func (t *tokenSimple) assignSimpleTokenToUser() {
	t.simpleTokensMu.Lock()
	runtime.Gosched()
	t.simpleTokenKeeper.addSimpleToken()
	t.simpleTokensMu.Unlock()
}
func newDeleterFunc(t *tokenSimple) func(string) {
	return func(tk string) {
		t.simpleTokensMu.Lock()
		defer t.simpleTokensMu.Unlock()
	}
}

func (t *tokenSimple) enable() {
	t.simpleTokenKeeper = NewSimpleTokenTTLKeeper(newDeleterFunc(t))
}

func (t *tokenSimple) disable() {
	if t.simpleTokenKeeper != nil {
		t.simpleTokenKeeper.stop()
		t.simpleTokenKeeper = nil
	}
	t.simpleTokensMu.Lock()
	t.simpleTokensMu.Unlock()
}

func newTokenProviderSimple() *tokenSimple {
	return &tokenSimple{}
}

func setupAuthStore() (store *authStore, teardownfunc func()) {
	as := &authStore{
		tokenProvider: newTokenProviderSimple(),
	}
	as.tokenProvider.enable()
	tearDown := func() {
		as.tokenProvider.disable()
	}
	return as, tearDown
}

///
///	G2										G1
///											stk.run()
///	ts.assignSimpleTokenToUser()
///	t.simpleTokensMu.Lock()
///	t.simpleTokenKeeper.addSimpleToken()
///	tm.addSimpleTokenCh <- true
///											<-tm.addSimpleTokenCh
///	t.simpleTokensMu.Unlock()
///	ts.assignSimpleTokenToUser()
///	...										...
///	t.simpleTokensMu.Lock()
///											<-tokenTicker.C
///	tm.addSimpleTokenCh <- true
///											tm.deleteTokenFunc()
///											t.simpleTokensMu.Lock()
///------------------------------------G1,G2 deadlock---------------------------------------------
///

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
	for i := 0; i < 100; i++ {
		go func() {
			// deadlocks: x > 0
			as, tearDown := setupAuthStore()
			defer tearDown()
			var wg sync.WaitGroup
			wg.Add(3)
			for i := 0; i < 3; i++ {
				go func() { // G2
					// deadlocks: x > 0
					as.Authenticate()
					defer wg.Done()
				}()
			}
			wg.Wait()
		}()
	}
}
