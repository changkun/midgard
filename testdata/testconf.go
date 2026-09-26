// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package testdata points the tests at testdata/config.yml.
package testdata

import (
	"os"
	"path/filepath"
	"runtime"
)

// UseConfig sets MIDGARD_CONF to the test configuration, as an absolute path
// so that a test changing its working directory still finds it. Call it from
// TestMain, before the configuration is first read.
func UseConfig() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot locate testdata/config.yml")
	}
	os.Setenv("MIDGARD_CONF", filepath.Join(filepath.Dir(file), "config.yml"))
}
