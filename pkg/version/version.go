// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package version

import (
	"fmt"
	"runtime"
)

// These strings will be overwritten at built time.
var (
	GitVersion string
	GoVersion  = runtime.Version()
	BuildTime  string
)

// String returns a newline-terminated string describing the current
// version of the build.
func String() string {
	if GitVersion == "" {
		GitVersion = "devel"
	}
	str := fmt.Sprintf("Vrsion:      %s\n", GitVersion)
	str += fmt.Sprintf("Go version:  %s\n", GoVersion)
	if BuildTime != "" {
		str += fmt.Sprintf("Build time:  %s\n", BuildTime)
	}
	return str
}
