// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package service

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// userDirs points the user configuration and cache directories into a
// temporary one, and fakes systemctl, recording its calls.
func userDirs(t *testing.T, systemd bool) (calls *[]string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("installing refuses to run as root")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))

	calls = new([]string)
	savedCtl, savedHas := systemctl, hasSystemdUser
	systemctl = func(args ...string) error {
		*calls = append(*calls, strings.Join(args, " "))
		return nil
	}
	hasSystemdUser = func() bool { return systemd }
	t.Cleanup(func() { systemctl, hasSystemdUser = savedCtl, savedHas })
	return calls
}

func newTestService(t *testing.T) *linuxService {
	t.Helper()
	s, err := NewService("midgard-test", "midgard test", "the midgard test daemon", []string{"daemon", "run"})
	if err != nil {
		t.Fatal(err)
	}
	return s.(*linuxService)
}

func TestInstallUserUnit(t *testing.T) {
	calls := userDirs(t, true)
	s := newTestService(t)

	if err := s.Install(); err != nil {
		t.Fatal(err)
	}
	path, _ := s.unitPath()
	if !strings.HasPrefix(path, os.Getenv("XDG_CONFIG_HOME")) {
		t.Fatalf("the unit went to %s, not the user's configuration", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	unit := string(b)
	exe, _ := os.Executable()
	for _, want := range []string{
		"ExecStart=" + quote(exe) + " daemon run\n",
		"WantedBy=graphical-session.target",
		"PartOf=graphical-session.target",
		"Restart=on-failure",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("the unit lacks %q:\n%s", want, unit)
		}
	}
	if got := strings.Join(*calls, "; "); got != "daemon-reload; enable midgard-test.service" {
		t.Errorf("systemctl --user calls: %s", got)
	}
	if err := s.Install(); err == nil {
		t.Error("a second install succeeded over the first")
	}

	*calls = nil
	if err := s.Remove(); err != nil {
		t.Fatal(err)
	}
	if fileExists(path) {
		t.Error("the unit is still there after uninstall")
	}
	if got := strings.Join(*calls, "; "); got != "disable --now midgard-test.service; daemon-reload" {
		t.Errorf("systemctl --user calls on uninstall: %s", got)
	}
	if err := s.Remove(); err == nil {
		t.Error("uninstalling twice succeeded")
	}
}

func TestInstallAutostartWithoutSystemd(t *testing.T) {
	calls := userDirs(t, false)
	s := newTestService(t)

	if err := s.Install(); err != nil {
		t.Fatal(err)
	}
	path, _ := s.autostartPath()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	if !strings.Contains(string(b), "Exec="+quote(exe)+" daemon run\n") {
		t.Errorf("the autostart entry does not run the daemon:\n%s", b)
	}
	if len(*calls) != 0 {
		t.Errorf("systemctl was called without systemd: %v", *calls)
	}
	if err := s.Remove(); err != nil || fileExists(path) {
		t.Fatalf("uninstall: %v, entry left: %v", err, fileExists(path))
	}
}

func TestQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/usr/local/bin/mg":   "/usr/local/bin/mg",
		"/home/me/my apps/mg": `"/home/me/my apps/mg"`,
		`a"b`:                 `"a\"b"`,
		"":                    `""`,
	} {
		if got := quote(in); got != want {
			t.Errorf("quote(%q) = %s, want %s", in, got, want)
		}
	}
}

// TestRunStopsOnSIGTERM: systemd stops a unit with SIGTERM, which Run did
// not listen for, so the daemon was killed rather than stopped.
func TestRunStopsOnSIGTERM(t *testing.T) {
	userDirs(t, true)
	s := newTestService(t)

	stopped := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.Run(func() error { return nil }, func() error { close(stopped); return nil })
	}()
	// Wait for Run to record the pid, which it does before listening.
	path, _ := s.pidPath()
	for deadline := time.Now().Add(5 * time.Second); !fileExists(path); {
		if time.Now().After(deadline) {
			t.Fatal("Run never recorded its pid")
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond) // and to start listening

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("SIGTERM did not stop the daemon")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if fileExists(path) {
		t.Error("the pid file outlived the daemon")
	}
}
