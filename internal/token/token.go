// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package token issues and checks the per-device tokens that clients use in
// place of the server's password. Each device gets its own, so one can be
// revoked without changing the others, and no device has to hold the
// password that administers the server.
//
// Only a token's SHA-256 is stored, one per line, as
//
//	name <tab> hex(sha256(token)) <tab> creation time
//
// in a file readable by the server's user alone.
package token

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Prefix starts every token, so that one is recognizable in a config file or
// a leaked log.
const Prefix = "mgt_"

// Info describes an issued token, without the token itself.
type Info struct {
	Name    string
	Created time.Time
}

type entry struct {
	Info
	hash [sha256.Size]byte
}

// Store is the token file. It is read afresh on every check, so a token added
// or revoked with mg server token takes effect on the next request, without a
// restart. The file is a few lines long, and a revoked token must not live on
// in a cache.
type Store struct {
	path string

	mu      sync.Mutex
	entries []entry
}

// Open returns the store kept in the file at path, which need not exist yet.
func Open(path string) *Store { return &Store{path: path} }

// Add issues a token for the device called name and returns it. The token
// is shown only this once; the store keeps its hash.
func (s *Store) Add(name string) (string, error) {
	if err := validName(name); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return "", err
	}
	for _, e := range s.entries {
		if e.Name == name {
			return "", fmt.Errorf("%q already has a token; remove it first", name)
		}
	}

	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	tok := Prefix + base64.RawURLEncoding.EncodeToString(raw[:])
	s.entries = append(s.entries, entry{
		Info: Info{Name: name, Created: time.Now().UTC().Truncate(time.Second)},
		hash: sha256.Sum256([]byte(tok)),
	})
	return tok, s.save()
}

// Remove revokes the token of the device called name.
func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	for i, e := range s.entries {
		if e.Name == name {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("no token named %q", name)
}

// List returns the issued tokens, oldest first.
func (s *Store) List() ([]Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return nil, err
	}
	out := make([]Info, len(s.entries))
	for i, e := range s.entries {
		out[i] = e.Info
	}
	return out, nil
}

// Check reports the name of the device tok was issued to, or ok false if it
// was not issued or has been revoked.
func (s *Store) Check(tok string) (name string, ok bool) {
	if !strings.HasPrefix(tok, Prefix) {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return "", false
	}
	h := sha256.Sum256([]byte(tok))
	for _, e := range s.entries {
		if subtle.ConstantTimeCompare(h[:], e.hash[:]) == 1 {
			name, ok = e.Name, true
		}
	}
	return name, ok
}

// load reads the file.
func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		s.entries = nil
		return nil
	}
	if err != nil {
		return err
	}
	var entries []entry
	sc := bufio.NewScanner(bytes.NewReader(b))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			return fmt.Errorf("%s:%d: want name, hash and time separated by tabs", s.path, n)
		}
		var e entry
		e.Name = f[0]
		if h, err := hex.DecodeString(f[1]); err != nil || len(h) != sha256.Size {
			return fmt.Errorf("%s:%d: bad hash", s.path, n)
		} else {
			copy(e.hash[:], h)
		}
		if e.Created, err = time.Parse(time.RFC3339, f[2]); err != nil {
			return fmt.Errorf("%s:%d: bad time: %v", s.path, n, err)
		}
		entries = append(entries, e)
	}
	s.entries = entries
	return nil
}

// save writes the file, replacing it in one step so that a server reading it
// never sees half of it.
func (s *Store) save() error {
	var b strings.Builder
	b.WriteString("# midgard device tokens: name, sha256 of the token, created\n")
	for _, e := range s.entries {
		fmt.Fprintf(&b, "%s\t%x\t%s\n", e.Name, e.hash, e.Created.Format(time.RFC3339))
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".tokens-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

// validName accepts names that fit on one line of the file and read well in
// a listing: letters, digits, and - _ . only.
func validName(name string) error {
	if name == "" || len(name) > 64 {
		return errors.New("a device name must be 1 to 64 characters")
	}
	for _, r := range name {
		ok := r == '-' || r == '_' || r == '.' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return fmt.Errorf("invalid device name %q: use letters, digits, - _ and . only", name)
		}
	}
	return nil
}
