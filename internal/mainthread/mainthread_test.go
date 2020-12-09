// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// +build linux

package mainthread_test

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"changkun.de/x/midgard/internal/mainthread"
	"golang.org/x/sys/unix"
)

var initTid int

func init() {
	initTid = unix.Getpid()
}

func TestMain(m *testing.M) {
	mainthread.Init(func() { os.Exit(m.Run()) })
}

// TestMainThread is not designed to be executed on the main thread.
// This test tests the a call from this function that is invoked by
// mainthread.Call is either executed on the main thread or not.
func TestMainThread(t *testing.T) {
	var (
		nmains = uint64(0)
		ncalls = 100000
	)

	wg := sync.WaitGroup{}
	for i := 0; i < ncalls; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			mainthread.Call(func() {
				// Code inside this function is expecting to be executed
				// on the mainthread, this means the thread id should be
				// euqal to the initial process id.
				current := unix.Gettid()
				if current == initTid {
					return
				}
				t.Fatalf("call is not executed on the main thread, want %d, got %d", initTid, current)
			})
		}()
		go func() {
			defer wg.Done()
			if unix.Gettid() == initTid {
				atomic.AddUint64(&nmains, 1)
			}
		}()
	}
	wg.Wait()

	if nmains == uint64(ncalls) {
		t.Fatalf("all non main thread calls are executed on the main thread.")
	}
}
