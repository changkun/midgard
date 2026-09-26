// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestSave: the Mac app asks for the server and keeps it where mg looks,
// keeping an app token already there, readable by its user alone.
func TestSave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	t.Setenv("MIDGARD_CONF", "")
	t.Chdir(t.TempDir())
	home, _ := os.UserConfigDir()
	path := filepath.Join(home, "midgard", "config.yml")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte("domain: old.example\ntoken: mgt_keep\n"), 0o600)

	got, err := Save("new.example")
	if err != nil || got != path {
		t.Fatalf("Save = %q, %v", got, err)
	}
	c, err := read(path)
	if err != nil || c.Domain != "new.example" || c.Token != "mgt_keep" {
		t.Fatalf("saved %+v, %v", c, err)
	}
	if err := Load(); err != nil || Get().Domain != "new.example" {
		t.Fatalf("Load: %v, domain %q", err, Get().Domain)
	}
	if fi, _ := os.Stat(path); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}
}
