// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFind pins where the configuration is looked for, and in which order.
func TestFind(t *testing.T) {
	write := func(t *testing.T, path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("title: t\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// home points every OS's user configuration directory into dir.
	home := func(t *testing.T, dir string) string {
		t.Helper()
		t.Setenv("HOME", dir)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
		t.Setenv("AppData", filepath.Join(dir, "AppData"))
		ucd, err := os.UserConfigDir()
		if err != nil {
			t.Fatal(err)
		}
		return filepath.Join(ucd, "midgard", "config.yml")
	}

	t.Run("MIDGARD_CONF wins", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		write(t, "config.yml")
		want := filepath.Join(dir, "elsewhere.yml")
		write(t, want)
		t.Setenv("MIDGARD_CONF", want)

		if got, err := find(); err != nil || got != want {
			t.Fatalf("find() = %q, %v; want %q", got, err, want)
		}
	})
	t.Run("MIDGARD_CONF naming a missing file is an error", func(t *testing.T) {
		t.Chdir(t.TempDir())
		write(t, "config.yml") // must not be used instead
		t.Setenv("MIDGARD_CONF", filepath.Join(t.TempDir(), "missing.yml"))

		if got, err := find(); err == nil {
			t.Fatalf("find() = %q, want an error", got)
		}
	})
	t.Run("working directory before user directory", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		t.Setenv("MIDGARD_CONF", "")
		write(t, home(t, dir))
		write(t, "config.yml")

		if got, err := find(); err != nil || got != "config.yml" {
			t.Fatalf("find() = %q, %v; want config.yml", got, err)
		}
	})
	t.Run("user configuration directory", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		t.Setenv("MIDGARD_CONF", "")
		want := home(t, dir)
		write(t, want)

		if got, err := find(); err != nil || got != want {
			t.Fatalf("find() = %q, %v; want %q", got, err, want)
		}
	})
}
