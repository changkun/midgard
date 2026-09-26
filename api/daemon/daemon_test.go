// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"os"
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
func TestDeviceID(t *testing.T) {
	home(t)
	id, err := deviceID()
	if err != nil || len(id) != 32 {
		t.Fatalf("deviceID = %q, %v", id, err)
	}
	if again, _ := deviceID(); again != id {
		t.Fatalf("the id changed: %q, then %q", id, again)
	}
}

func TestHistoryPath(t *testing.T) {
	dir := home(t)
	path, err := historyPath()
	if err != nil || !strings.HasPrefix(path, dir) || filepath.Base(path) != "history.db" {
		t.Fatalf("historyPath = %q, %v; want it under %s", path, err, dir)
	}
	if runtime.GOOS == "linux" && !strings.HasPrefix(path, filepath.Join(dir, "data")) {
		t.Errorf("on Linux the history is data: %q", path)
	}
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
	if _, err := os.Stat(filepath.Dir(mustHistoryPath(t))); err != nil {
		t.Fatalf("no history: %v", err)
	}
}

func mustHistoryPath(t *testing.T) string {
	p, err := historyPath()
	if err != nil {
		t.Fatal(err)
	}
	return p
}
