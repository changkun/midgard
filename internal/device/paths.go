// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package device

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ID is this install's id, made once and kept in the user's configuration
// directory. It names the device to the server, which holds copies until
// each device has them (specs/redesign.md §6); a host name would make a
// reinstalled machine, or two with the same name, one device. The daemon
// and the Mac app are the same install, and share it.
func ID() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "midgard", "device")
	b, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(b))) > 0 {
		return strings.TrimSpace(string(b)), nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return id, os.WriteFile(path, []byte(id+"\n"), 0o600)
}

// HistoryPath is where the device keeps its history: the user's data
// directory, $XDG_DATA_HOME or ~/.local/share on Linux and the BSDs, and
// the configuration directory elsewhere, where the data goes too.
func HistoryPath() (string, error) {
	dir := ""
	switch runtime.GOOS {
	case "darwin", "windows", "ios", "android", "plan9":
		d, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = d
	default:
		dir = os.Getenv("XDG_DATA_HOME")
		if dir == "" || !filepath.IsAbs(dir) {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(dir, "midgard", "history.db"), nil
}

// Lock makes sure one midgard syncs this device at a time, the daemon or the
// Mac app: two would be the same device to the server, each taking the other's
// connection. It holds until the process ends, or release is called.
func Lock() (release func(), err error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "midgard", "device.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, ErrRunning
	}
	return func() { f.Close() }, nil
}

// ErrRunning is returned by Lock when another midgard syncs this device.
var ErrRunning = errors.New("midgard already runs on this device, as the daemon or the app; stop it first")
