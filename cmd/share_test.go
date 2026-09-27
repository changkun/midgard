// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestShareRefusesCommandLikeNames: mg share list, typed by one who means to
// list their shares, published the clipboard at /midgard/list. It refuses
// such a name, before it reads the clipboard, and says what to type.
//
// It re-runs the test binary, as mg exits the process when it fails.
func TestShareRefusesCommandLikeNames(t *testing.T) {
	if name := os.Getenv("MIDGARD_TEST_SHARE"); name != "" {
		os.Args = []string{"mg", "share", name}
		Execute()
		return
	}

	for _, name := range []string{"list", "ls", "rm"} {
		home := t.TempDir()
		conf := filepath.Join(home, "config.yml")
		if err := os.WriteFile(conf, []byte("domain: http://127.0.0.1:1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestShareRefusesCommandLikeNames$")
		cmd.Dir = home
		cmd.Env = append(os.Environ(), "MIDGARD_TEST_SHARE="+name, "MIDGARD_CONF="+conf,
			"HOME="+home, "XDG_CONFIG_HOME="+home, "XDG_DATA_HOME="+home)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != exitUsage {
			t.Errorf("mg share %s: %v, want exit %d\n%s", name, err, exitUsage, out)
		}
		if !strings.Contains(string(out), "mg shares") {
			t.Errorf("mg share %s does not say to use mg shares:\n%s", name, out)
		}
	}
}
