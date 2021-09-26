// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package version

import (
	"fmt"
	"runtime"
	"strings"
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
	var B = new(strings.Builder)
	fmt.Fprintf(B, "Version:     %s\n", GitVersion)
	fmt.Fprintf(B, "Go version:  %s\n", GoVersion)
	if BuildTime != "" {
		fmt.Fprintf(B, "Build time:  %s\n", BuildTime)
	}
	return B.String()
}
