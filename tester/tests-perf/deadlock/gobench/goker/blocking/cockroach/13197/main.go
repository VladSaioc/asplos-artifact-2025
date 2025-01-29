/*
 * Project: cockroach
 * Issue or PR  : https://github.com/cockroachdb/cockroach/pull/13197
 * Buggy version: fff27aedabafe20cef57f75905fe340cab48c2a4
 * fix commit-id: 9bf770cd8f6eaff5441b80d3aec1a5614e8747e1
 * Flaky: 100/100
 * Description: One goroutine executing (*Tx).awaitDone() blocks and
 * waiting for a signal context.Done().
 */
package main

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type DB struct{}

func (db *DB) begin(ctx context.Context) *Tx {
	ctx, cancel := context.WithCancel(ctx)
	tx := &Tx{
		cancel: cancel,
		ctx:    ctx,
	}
	// deadlocks: x > 0
	go tx.awaitDone() // G2
	return tx
}

type Tx struct {
	cancel context.CancelFunc
	ctx    context.Context
}

func (tx *Tx) awaitDone() {
	<-tx.ctx.Done()
}

func (tx *Tx) Rollback() {
	tx.rollback()
}

func (tx *Tx) rollback() {
	tx.close()
}

func (tx *Tx) close() {
	tx.cancel()
}

/// G1 				G2
/// begin()
/// 				awaitDone()
/// 				<-tx.ctx.Done()
/// return
/// -----------G2 leak-------------

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
					// deadlocks: x > 0
					db := &DB{}
					db.begin(context.Background()) // G1
				}()
			}
		}
	}()
}
