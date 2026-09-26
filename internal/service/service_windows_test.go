// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package service

import (
	"os"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// TestInstallRunKey: the daemon is started at logon from the per-user Run key
// rather than installed as a Windows service, whose Session 0 clipboard no one
// in the user's session can see.
func TestInstallRunKey(t *testing.T) {
	s, err := NewService("midgard-test", "midgard test", "the midgard test daemon", []string{"daemon", "run"})
	if err != nil {
		t.Fatal(err)
	}
	ws := s.(*windowsService)
	t.Cleanup(func() { ws.Remove() })

	if err := ws.Install(); err != nil {
		t.Fatal(err)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	got, _, err := k.GetStringValue("midgard-test")
	if err != nil {
		t.Fatalf("no Run value after install: %v", err)
	}
	exe, _ := os.Executable()
	if want := syscall.EscapeArg(exe) + " daemon run"; got != want {
		t.Fatalf("Run value = %q, want %q", got, want)
	}
	if ws.legacyInstalled() {
		t.Error("install created a Windows service")
	}
	if err := ws.Install(); err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Errorf("a second install: %v, want already installed", err)
	}

	if err := ws.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := k.GetStringValue("midgard-test"); err == nil {
		t.Error("the Run value survived uninstall")
	}
	if err := ws.Remove(); err == nil {
		t.Error("uninstalling twice succeeded")
	}
}
