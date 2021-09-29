// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package version

import (
	"fmt"
	"runtime"
	"strings"
)

// Version is a newline-terminated string describing the current
// version of the build.
var Version string

// These strings will be overwritten at built time.
var (
	GitVersion string
	GoVersion  = runtime.Version()
	BuildTime  string
)

func init() {
	if GitVersion == "" {
		GitVersion = "devel"
	}
	var b = new(strings.Builder)
	fmt.Fprintf(b, "Version:     %s\n", GitVersion)
	fmt.Fprintf(b, "Go version:  %s\n", GoVersion)
	if BuildTime != "" {
		fmt.Fprintf(b, "Build time:  %s\n", BuildTime)
	}
	Version = b.String()
}
