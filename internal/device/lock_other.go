// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

//go:build plan9 || js || wasip1

package device

import "os"

// lockFile takes no lock where there is none to take.
func lockFile(*os.File) error { return nil }
