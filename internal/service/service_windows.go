// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package service

// On Windows the daemon starts when the user logs in, from the per-user Run
// key, and runs in their session. It used to be a Windows service, and a
// service runs in Session 0, whose clipboard nothing in the user's session
// can see: the daemon synced a clipboard nobody used, and worked only while a
// terminal ran it (#29). A service installed that way is still recognized,
// run, and removed by mg daemon uninstall.

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// runKey is the per-user list of programs Windows starts at logon.
const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func newService(c *config) (*windowsService, error) {
	return &windowsService{
		name:        c.Name,
		displayName: c.DisplayName,
		description: c.Description,
		args:        c.Args,
	}, nil
}

type windowsService struct {
	name, displayName, description string
	onStart, onStop                func() error
	args                           []string
	logger                         *eventlog.Log // only when run as a legacy service
}

// command is the daemon's command line as the Run key holds it.
func (ws *windowsService) command() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	words := []string{syscall.EscapeArg(exe)}
	for _, a := range ws.args {
		words = append(words, syscall.EscapeArg(a))
	}
	return strings.Join(words, " "), nil
}

func (ws *windowsService) pidPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "midgard", ws.name+".pid"), nil
}

func (ws *windowsService) Install() error {
	if ws.legacyInstalled() {
		slog.Warn("an older daemon is installed as a Windows service, which cannot reach your " +
			"clipboard; remove it with mg daemon uninstall, in a PowerShell run as administrator")
	}
	cmd, err := ws.command()
	if err != nil {
		return err
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if _, _, err := k.GetStringValue(ws.name); err == nil {
		return fmt.Errorf("%s is already installed", ws.name)
	}
	slog.Info("adding to the programs started at logon", "key", `HKCU\`+runKey, "name", ws.name)
	return k.SetStringValue(ws.name, cmd)
}

func (ws *windowsService) Remove() error {
	removed := false
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if _, _, err := k.GetStringValue(ws.name); err == nil {
		if err := k.DeleteValue(ws.name); err != nil {
			return err
		}
		removed = true
	}
	if ws.legacyInstalled() {
		if err := ws.removeLegacy(); err != nil {
			return fmt.Errorf("cannot remove the older daemon installed as a Windows service "+
				"(run PowerShell as administrator): %w", err)
		}
		removed = true
	}
	if !removed {
		return fmt.Errorf("%s is not installed", ws.name)
	}
	return nil
}

// legacyInstalled reports whether an older mg daemon install left the daemon
// registered as a Windows service. Asking needs no administrator rights.
func (ws *windowsService) legacyInstalled() bool {
	m, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return false
	}
	defer windows.CloseServiceHandle(m)
	name, err := windows.UTF16PtrFromString(ws.name)
	if err != nil {
		return false
	}
	s, err := windows.OpenService(m, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return false
	}
	windows.CloseServiceHandle(s)
	return true
}

// removeLegacy stops and deletes the service an older install created. It
// needs administrator rights.
func (ws *windowsService) removeLegacy() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(ws.name)
	if err != nil {
		return err
	}
	defer s.Close()
	s.Control(svc.Stop) // it may not be running
	if err := s.Delete(); err != nil {
		return err
	}
	eventlog.Remove(ws.name)
	slog.Info("removed the older daemon installed as a Windows service")
	return nil
}

func (ws *windowsService) Start() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Detached, without a console: the daemon outlives this terminal, and
	// has no window of its own.
	cmd := exec.Command(exe, ws.args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd.Start()
}

func (ws *windowsService) Stop() error {
	path, err := ws.pidPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s does not seem to be running: %w", ws.name, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := p.Kill(); err != nil {
		return err
	}
	os.Remove(path)
	return nil
}

func (ws *windowsService) Run(onStart, onStop func() error) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if isService {
		return ws.runIsService(onStart, onStop)
	}
	return ws.runNotService(onStart, onStop)
}

func (ws *windowsService) runNotService(onStart, onStop func() error) error {
	// Started at logon, the daemon gets a console window of its own; close
	// it. Started from a terminal, it shares that terminal's console, which
	// stays.
	if consoleIsOurs() {
		freeConsole.Call()
	}
	if path, err := ws.pidPath(); err == nil {
		if os.MkdirAll(filepath.Dir(path), 0o700) == nil {
			os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600)
			defer os.Remove(path)
		}
	}

	err := onStart()
	if err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	return onStop()
}

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	freeConsole           = kernel32.NewProc("FreeConsole")
	getConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
)

// consoleIsOurs reports whether this process is the only one attached to its
// console, which is the case when Windows made the console for it.
func consoleIsOurs() bool {
	var pids [2]uint32
	n, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return n == 1
}

// runIsService runs the daemon as the Windows service an older install
// created, until it is removed.
func (ws *windowsService) runIsService(onStart, onStop func() error) error {
	if elog, err := eventlog.Open(ws.name); err == nil {
		defer elog.Close()
		ws.logger = elog
	}
	ws.onStart = onStart
	ws.onStop = onStop
	return svc.Run(ws.name, ws)
}

func (ws *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	if err := ws.onStart(); err != nil {
		ws.Error("%v", err)
		return true, 1
	}

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for {
		c := <-r
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			if err := ws.onStop(); err != nil {
				ws.Error("%v", err)
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
				continue loop
			}
			break loop
		default:
			continue loop
		}
	}

	return
}

func (ws *windowsService) Error(format string, a ...any) error {
	if ws.logger == nil {
		slog.Error(fmt.Sprintf(format, a...))
		return nil
	}
	return ws.logger.Error(3, fmt.Sprintf(format, a...))
}

func (ws *windowsService) Warning(format string, a ...any) error {
	if ws.logger == nil {
		slog.Warn(fmt.Sprintf(format, a...))
		return nil
	}
	return ws.logger.Warning(2, fmt.Sprintf(format, a...))
}

func (ws *windowsService) Info(format string, a ...any) error {
	if ws.logger == nil {
		slog.Info(fmt.Sprintf(format, a...))
		return nil
	}
	return ws.logger.Info(1, fmt.Sprintf(format, a...))
}
