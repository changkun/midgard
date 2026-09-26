// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"changkun.de/x/midgard/internal/device"
)

// home gives the test a home of its own, touching nothing of the user's.
func home(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("AppData", filepath.Join(dir, "appdata"))
	t.Setenv("MIDGARD_CONF", "")
	t.Chdir(t.TempDir())
}

// TestEngine: the app sets the server on its first start, then syncs; what
// is copied while the server cannot be reached waits, and the list shows it.
func TestEngine(t *testing.T) {
	home(t)
	if err := start(nil); !errors.Is(err, errNoServer) {
		t.Fatalf("start before the server is set: %v, want errNoServer", err)
	}
	if s := statusNow(); s.Configured || s.Running {
		t.Fatalf("status before setup: %+v", s)
	}
	if err := setup("http://127.0.0.1:1"); err != nil { // nothing answers there
		t.Fatal(err)
	}
	if err := start(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdown)
	s := statusNow()
	if !s.Configured || s.Server != "http://127.0.0.1:1" || !s.Running || s.Online || s.SignedIn || s.Device == "" {
		t.Fatalf("status %+v", s)
	}

	// one midgard syncs a device at a time
	if _, err := device.Lock(); !errors.Is(err, device.ErrRunning) {
		t.Fatalf("mg daemon while the app runs: %v, want ErrRunning", err)
	}

	if err := copyData("text", []byte(strings.Repeat("é", previewLen))); err != nil {
		t.Fatal(err)
	}
	if err := copyData("text", nil); err == nil {
		t.Error("copied nothing")
	}
	b, err := historyJSON(10)
	if err != nil {
		t.Fatal(err)
	}
	var list []entry
	json.Unmarshal(b, &list)
	if len(list) != 1 || !list[0].Waiting || list[0].MIME != "text" || list[0].Size != 2*previewLen {
		t.Fatalf("history %s", b)
	}
	if _, _, err := get(1); err == nil {
		t.Error("got a copy nobody numbered")
	}

	shutdown()
	if _, err := historyJSON(10); !errors.Is(err, errNotStarted) {
		t.Fatalf("after stopping: %v", err)
	}
	// stopped, the device is free for the daemon
	release, err := device.Lock()
	if err != nil {
		t.Fatalf("after stopping, the lock: %v", err)
	}
	release()
}

func TestPreview(t *testing.T) {
	long := strings.Repeat("a", previewLen-1) + "é" // é is two bytes, across the cut
	if p := preview([]byte(long)); p != strings.Repeat("a", previewLen-1) {
		t.Fatalf("preview cut inside a rune: %q", p[len(p)-3:])
	}
	if p := preview([]byte("short")); p != "short" {
		t.Fatalf("preview %q", p)
	}
}
