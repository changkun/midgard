// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package main

import (
	"changkun.de/x/midgard/cmd"
	"golang.design/x/mainthread"
)

func main() {
	// midgard cli involes graphical APIs that require midgard daemon to
	// running on the main thread. Instead of executing the command center,
	// initialize it from the golang.design/x/mainthread package.
	mainthread.Init(cmd.Execute)
}
