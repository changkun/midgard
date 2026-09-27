// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package device

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"changkun.de/x/midgard/internal/e2e"
)

// A device keeps its person's key beside its sign-in (specs/redesign.md
// §11), in key.json in midgard's configuration directory, readable by its
// person alone. The daemon, the Mac app and mg on one machine share it.

// keyFile is what key.json holds: the key, and the last seq numbered before
// it was made, below which a copy in the clear is the person's history from
// before the key (e2e.Code.Seal).
type keyFile struct {
	Key   []byte `json:"key"`
	Since uint64 `json:"since"`
}

// KeyPath is where this device keeps its person's key.
func KeyPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "midgard", "key.json"), nil
}

// LoadKey is the key this device keeps, and since; nil when it has none.
func LoadKey() (*e2e.Key, uint64, error) {
	path, err := KeyPath()
	if err != nil {
		return nil, 0, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	var f keyFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, 0, err
	}
	k, err := e2e.KeyFrom(f.Key)
	return k, f.Since, err
}

// SaveKey keeps k and since, in place of what was kept: whole, or not at
// all, as a device without its key cannot read its person's copies.
func SaveKey(k *e2e.Key, since uint64) error {
	path, err := KeyPath()
	if err != nil {
		return err
	}
	b, err := json.Marshal(keyFile{Key: k.Raw(), Since: since})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
