// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestVersionNeedsNoConfig runs mg version where no configuration can be
// found. It used to exit before printing anything, because the endpoints were
// computed from the configuration when the program started, and the server's
// Chrome check panicked in every command. A command that needs neither must
// not depend on them.
//
// It re-runs the test binary so that the configuration lookup, which exits
// the process when it fails, cannot take the test runner down with it.
func TestVersionNeedsNoConfig(t *testing.T) {
	if os.Getenv("MIDGARD_TEST_VERSION") == "1" {
		os.Args = []string{"mg", "version"}
		Execute()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestVersionNeedsNoConfig$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(),
		"MIDGARD_TEST_VERSION=1",
		"MIDGARD_CONF="+cmd.Dir+"/missing.yml",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mg version failed without a configuration: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Version:") {
		t.Fatalf("mg version printed no version:\n%s", out)
	}
}
