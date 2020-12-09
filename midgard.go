// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package main

import (
	"changkun.de/x/midgard/cmd"
	"changkun.de/x/midgard/internal/mainthread"
)

func main() {
	// midgard cli involes graphical APIs that require running on
	// the main thread. Instead of executing the command center,
	// initialize it from the mainthread package.
	mainthread.Init(cmd.Execute)
}
