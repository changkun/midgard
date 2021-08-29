// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package osext

import "path/filepath"

var cx, ce = executableClean()

func executableClean() (string, error) {
	p, err := executable()
	return filepath.Clean(p), err
}

// Executable returns an absolute path that can be used to
// re-invoke the current program.
// It may not be valid after the current program exits.
func Executable() (string, error) {
	return cx, ce
}
