// Copyright 2020-2021 The golang.design Initiative Authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.
//
// Written by Changkun Ou <changkun.de>

// Package mainthread offers facilities to schedule functions
// on the main thread. To use this package properly, one must
// call `mainthread.Init` from the main package. For example:
//
// 	package main
//
// 	import "golang.design/x/mainthread"
//
// 	func main() { mainthread.Init(fn) }
//
// 	// fn is the actual main function
// 	func fn() {
// 		// ... do whatever you want to do ...
//
// 		// mainthread.Call returns when f1 returns. Note that if f1
// 		// blocks it will also block the execution of any subsequent
// 		// calls on the main thread.
// 		mainthread.Call(f1)
//
// 		// ... do whatever you want to do ...
//
// 		// mainthread.Go returns immediately and f2 is scheduled to be
// 		// executed in the future.
// 		mainthread.Go(f2)
//
// 		// ... do whatever you want to do ...
// 	}
//
// 	func f1() { ... }
// 	func f2() { ... }
package mainthread // import "golang.design/x/mainthread"

import (
	"runtime"
	"sync"
)

func init() {
	runtime.LockOSThread()
}

// Init initializes the functionality of running arbitrary subsequent
// functions be called on the main system thread.
//
// Init must be called in the main.main function.
func Init(main func()) {
	done := donePool.Get().(chan struct{})
	defer donePool.Put(done)

	go func() {
		defer func() {
			done <- struct{}{}
		}()
		main()
	}()

	for {
		select {
		case f := <-funcQ:
			f.fn()
			if f.done != nil {
				f.done <- struct{}{}
			}
		case <-done:
			return
		}
	}
}

// Call calls f on the main thread and blocks until f finishes.
func Call(f func()) {
	done := donePool.Get().(chan struct{})
	defer donePool.Put(done)

	funcQ <- funcData{fn: f, done: done}
	<-done
}

// Go schedules f to be called on the main thread.
func Go(f func()) {
	funcQ <- funcData{fn: f}
}

var (
	funcQ    = make(chan funcData, runtime.GOMAXPROCS(0))
	donePool = sync.Pool{New: func() interface{} {
		return make(chan struct{})
	}}
)

type funcData struct {
	fn   func()
	done chan struct{}
}
