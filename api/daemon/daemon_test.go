// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"changkun.de/x/midgard/internal/device"
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

func TestNewDaemon(t *testing.T) {
	home(t)
	m, err := NewDaemon()
	if err != nil {
		t.Fatal(err)
	}
	defer m.engine.History.Close()
	if m.engine.ID == "" || m.engine.Name == "" {
		t.Fatalf("the daemon is %+v", m.engine)
	}
	path, _ := device.HistoryPath()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no history: %v", err)
	}
	// one midgard syncs a device at a time
	if _, err := NewDaemon(); !errors.Is(err, device.ErrRunning) {
		t.Fatalf("a second daemon: %v, want ErrRunning", err)
	}
}
