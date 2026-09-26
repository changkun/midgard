// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

//go:build !windows && !plan9 && !js && !wasip1

package device

import (
	"os"
	"syscall"
)

// lockFile takes f's lock, which the system gives up when the process ends.
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
