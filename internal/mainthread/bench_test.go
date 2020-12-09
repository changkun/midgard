// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package mainthread_test

import (
	"fmt"
	"testing"

	"changkun.de/x/midgard/internal/mainthread"
)

func BenchmarkCall(b *testing.B) {
	f1 := func() {}
	f2 := func() {}

	mainthread.Init(func() {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if i%2 == 0 {
				mainthread.Call(f1)
			} else {
				mainthread.Call(f2)
			}
		}
	})
}

func ExampleInit() {
	mainthread.Init(func() {
		mainthread.Call(func() {
			fmt.Println("from main thread")
		})
	})
	// Output: from main thread
}
