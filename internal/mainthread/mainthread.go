// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package mainthread

import "runtime"

var funcQ chan func()

func init() {
	runtime.LockOSThread()
	funcQ = make(chan func(), runtime.GOMAXPROCS(0))
}

// Init initializes the functionality for running arbitrary subsequent
// functions on the main system thread.
//
// Init must be called in the main package.
func Init(run func()) {
	done := make(chan struct{})
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		run()
	}()

	for {
		select {
		case f := <-funcQ:
			f()
		case <-done:
			return
		}
	}
}

// Call calls f on the main thread and blocks until f finishes.
func Call(f func()) {
	done := make(chan struct{})
	funcQ <- func() {
		defer func() {
			done <- struct{}{}
		}()
		f()
	}
	<-done
}
