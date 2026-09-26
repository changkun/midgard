// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package service

// On Linux the daemon is installed for the user who runs mg daemon install,
// not for the system. A clipboard belongs to a desktop session, and a system
// service — started by root at boot, before anyone logs in — has no session
// and so no clipboard to reach; that is why the daemon used to work only
// while a terminal ran it.
//
// With a systemd user manager, the daemon is a user unit bound to the
// graphical session, so it starts with the desktop and sees its DISPLAY or
// WAYLAND_DISPLAY. Without one, it is an XDG autostart entry, which desktops
// run at login.

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/template"
)

// systemctl runs systemctl --user; a variable so the tests need no systemd.
var systemctl = func(args ...string) error {
	out, err := exec.Command("systemctl", append([]string{"--user"}, args...)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl --user %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return nil
}

// hasSystemdUser reports whether a systemd user manager is running for this
// user; a variable so the tests can choose.
var hasSystemdUser = func() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	return systemctl("show-environment") == nil
}

// legacyUnits are where mg daemon install used to put a system-wide service.
var legacyUnits = []string{
	"/etc/systemd/system/%s.service",
	"/etc/init.d/%s",
	"/etc/init/%s.conf",
}

type linuxService struct {
	name, displayName, description string
	args                           []string
}

func newService(c *config) (Service, error) {
	return &linuxService{
		name:        c.Name,
		displayName: c.DisplayName,
		description: c.Description,
		args:        c.Args,
	}, nil
}

func (s *linuxService) unitPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "systemd", "user", s.name+".service"), nil
}

func (s *linuxService) autostartPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", s.name+".desktop"), nil
}

// pidPath is where Run records the daemon's process, so Stop can find one
// started from an autostart entry.
func (s *linuxService) pidPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "midgard", s.name+".pid"), nil
}

// command is the daemon's command line, quoted for a unit's ExecStart and a
// desktop entry's Exec, which quote the same way.
func (s *linuxService) command() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot locate the %s executable: %w", s.name, err)
	}
	words := []string{quote(exe)}
	for _, a := range s.args {
		words = append(words, quote(a))
	}
	return strings.Join(words, " "), nil
}

// quote wraps a word in double quotes if it needs them.
func quote(w string) string {
	if w != "" && !strings.ContainsAny(w, " \t\n\"'\\$`;&|<>()*?[]#~%") {
		return w
	}
	return strconv.Quote(w)
}

func (s *linuxService) Install() error {
	if os.Geteuid() == 0 {
		return errors.New("install the daemon as the user whose clipboard it syncs, without sudo: " +
			"a service run by root has no desktop session, so no clipboard to reach")
	}
	s.warnLegacy()

	cmd, err := s.command()
	if err != nil {
		return err
	}
	data := struct{ Display, Description, Command string }{s.displayName, s.description, cmd}

	if !hasSystemdUser() {
		path, err := s.autostartPath()
		if err != nil {
			return err
		}
		if err := writeNew(path, autostartEntry, data); err != nil {
			return err
		}
		slog.Info("installed an autostart entry; the daemon starts when you next log in", "path", path)
		return nil
	}

	path, err := s.unitPath()
	if err != nil {
		return err
	}
	if err := writeNew(path, userUnit, data); err != nil {
		return err
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	return systemctl("enable", s.name+".service")
}

// writeNew renders tmpl into a file at path that must not exist yet.
func writeNew(path, tmpl string, data any) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("already installed: %s", path)
	}
	var b bytes.Buffer
	if err := template.Must(template.New("").Parse(tmpl)).Execute(&b, data); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	slog.Info("creating", "path", path)
	return os.WriteFile(path, b.Bytes(), 0o644)
}

func (s *linuxService) Remove() error {
	removed := false
	if path, err := s.unitPath(); err == nil && fileExists(path) {
		systemctl("disable", "--now", s.name+".service") // may already be stopped
		if err := os.Remove(path); err != nil {
			return err
		}
		systemctl("daemon-reload")
		removed = true
	}
	if path, err := s.autostartPath(); err == nil {
		if err := os.Remove(path); err == nil {
			removed = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if s.removeLegacy() {
		removed = true
	}
	if !removed {
		return fmt.Errorf("%s is not installed", s.name)
	}
	return nil
}

// warnLegacy points out a system-wide service left by an older mg daemon
// install. It still starts at boot, as root, syncing no one's clipboard.
func (s *linuxService) warnLegacy() {
	for _, p := range legacyUnits {
		if p := fmt.Sprintf(p, s.name); fileExists(p) {
			slog.Warn("an older, system-wide daemon is installed and cannot reach your clipboard; "+
				"remove it with: sudo mg daemon uninstall", "path", p)
		}
	}
}

// removeLegacy removes the system-wide service an older mg daemon install
// left, which takes root. It reports whether there was one to remove.
func (s *linuxService) removeLegacy() bool {
	found := false
	for _, p := range legacyUnits {
		p = fmt.Sprintf(p, s.name)
		if !fileExists(p) {
			continue
		}
		found = true
		if os.Geteuid() != 0 {
			slog.Warn("an older, system-wide daemon is installed; remove it with: sudo mg daemon uninstall", "path", p)
			continue
		}
		if strings.HasPrefix(p, "/etc/systemd/") {
			exec.Command("systemctl", "disable", "--now", s.name+".service").Run()
		}
		if err := os.Remove(p); err != nil {
			slog.Error("cannot remove the old daemon", "path", p, "err", err)
			continue
		}
		slog.Info("removed the old system-wide daemon", "path", p)
	}
	if found && os.Geteuid() == 0 {
		exec.Command("systemctl", "daemon-reload").Run()
	}
	return found
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (s *linuxService) Start() error {
	if path, err := s.unitPath(); err == nil && fileExists(path) {
		return systemctl("start", s.name+".service")
	}
	path, err := s.autostartPath()
	if err != nil || !fileExists(path) {
		return fmt.Errorf("%s is not installed; run mg daemon install first", s.name)
	}
	// An autostart entry starts at login only; start this session's now.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, s.args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // outlive this terminal
	return cmd.Start()
}

func (s *linuxService) Stop() error {
	if path, err := s.unitPath(); err == nil && fileExists(path) {
		return systemctl("stop", s.name+".service")
	}
	path, err := s.pidPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s does not seem to be running: %w", s.name, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

func (s *linuxService) Run(onStart, onStop func() error) (err error) {
	if path, err := s.pidPath(); err == nil {
		if os.MkdirAll(filepath.Dir(path), 0o700) == nil {
			os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600)
			defer os.Remove(path)
		}
	}

	err = onStart()
	if err != nil {
		return err
	}
	defer func() {
		err = onStop()
	}()

	// SIGTERM is how systemd, and Stop, ask the daemon to stop; os.Kill
	// was listed here before, but a process cannot catch it.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	return nil
}

func (s *linuxService) Error(format string, a ...any) error {
	slog.Error(fmt.Sprintf(format, a...))
	return nil
}

func (s *linuxService) Warning(format string, a ...any) error {
	slog.Warn(fmt.Sprintf(format, a...))
	return nil
}

func (s *linuxService) Info(format string, a ...any) error {
	slog.Info(fmt.Sprintf(format, a...))
	return nil
}

// userUnit starts with the graphical session, which is when a clipboard
// exists. GNOME and KDE start graphical-session.target; under a compositor
// that does not, such as a bare sway, start the unit from its configuration.
const userUnit = `[Unit]
Description={{.Description}}
PartOf=graphical-session.target
After=graphical-session.target

[Service]
ExecStart={{.Command}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=graphical-session.target
`

const autostartEntry = `[Desktop Entry]
Type=Application
Name={{.Display}}
Comment={{.Description}}
Exec={{.Command}}
NoDisplay=true
X-GNOME-Autostart-enabled=true
`
