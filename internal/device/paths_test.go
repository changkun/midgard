// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package device

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// home gives the test a home, and a configuration and data directory, of its
// own, so it touches nothing of the user's.
func home(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("AppData", filepath.Join(dir, "appdata"))
	return dir
}

// TestDeviceID: an install is one device, by an id it keeps; the host name
// named it before, and a reinstall or a second machine of the same name was
// the same device.
func TestID(t *testing.T) {
	home(t)
	id, err := ID()
	if err != nil || len(id) != 32 {
		t.Fatalf("ID = %q, %v", id, err)
	}
	if again, _ := ID(); again != id {
		t.Fatalf("the id changed: %q, then %q", id, again)
	}
}

func TestHistoryPath(t *testing.T) {
	dir := home(t)
	path, err := HistoryPath()
	if err != nil || !strings.HasPrefix(path, dir) || filepath.Base(path) != "history.db" {
		t.Fatalf("historyPath = %q, %v; want it under %s", path, err, dir)
	}
	if runtime.GOOS == "linux" && !strings.HasPrefix(path, filepath.Join(dir, "data")) {
		t.Errorf("on Linux the history is data: %q", path)
	}
}

// TestLock: one midgard syncs a device at a time, the daemon or the app.
func TestLock(t *testing.T) {
	home(t)
	release, err := Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(); !errors.Is(err, ErrRunning) {
		t.Fatalf("a second lock: %v, want ErrRunning", err)
	}
	release()
	again, err := Lock()
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	again()
}
