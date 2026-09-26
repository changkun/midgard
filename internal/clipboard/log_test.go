// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/testdata"
)

func TestMain(m *testing.M) {
	testdata.UseConfig()
	os.Exit(m.Run())
}

// logged returns the clipboard log files under the working directory.
func logged(t *testing.T) []string {
	t.Helper()
	var files []string
	filepath.WalkDir("data/logs", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	return files
}

// TestUniversalLogIsOptIn pins that the server does not keep what is copied
// unless configured to. It used to write every text, passwords included, to
// data/logs in plain text, world-writable.
func TestUniversalLogIsOptIn(t *testing.T) {
	t.Chdir(t.TempDir())

	clipboard.UniversalFor("test-owner").Write(types.MIMEPlainText, []byte("secret-one"))
	if files := logged(t); len(files) != 0 {
		t.Fatalf("the clipboard was logged without log_clipboard: %v", files)
	}

	config.S().Store.LogClipboard = true
	t.Cleanup(func() { config.S().Store.LogClipboard = false })

	clipboard.UniversalFor("test-owner").Write(types.MIMEPlainText, []byte("secret-two"))
	files := logged(t)
	if len(files) != 1 {
		t.Fatalf("log_clipboard is set, want one log file, got %v", files)
	}
	if runtime.GOOS == "windows" {
		return // Windows has no Unix permission bits to check
	}
	fi, err := os.Stat(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("clipboard log mode = %v, want 0600", fi.Mode().Perm())
	}
}
