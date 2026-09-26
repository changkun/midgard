// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data", "midgard.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestAppTokens(t *testing.T) {
	s, path := open(t)
	ctx := context.Background()

	phone, err := s.IssueAppToken(ctx, "alice", "phone")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(phone, TokenPrefix) {
		t.Fatalf("token %q lacks the %q prefix", phone, TokenPrefix)
	}

	// A second handle on the file is the server while mg server token
	// runs: it sees an issued token, and a revoked one goes at once.
	server, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if owner, name, ok := server.CheckAppToken(ctx, phone); !ok || owner != "alice" || name != "phone" {
		t.Fatalf("CheckAppToken = %q, %q, %v; want alice's phone", owner, name, ok)
	}
	for _, bad := range []string{"", TokenPrefix, TokenPrefix + "nope", phone + "x", strings.TrimPrefix(phone, TokenPrefix)} {
		if owner, _, ok := server.CheckAppToken(ctx, bad); ok {
			t.Errorf("CheckAppToken(%q) accepted it for %q", bad, owner)
		}
	}

	if _, err := s.IssueAppToken(ctx, "alice", "phone"); err == nil {
		t.Error("a second token under the same name was issued")
	}
	if err := s.RevokeAppToken(ctx, "alice", "phone"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := server.CheckAppToken(ctx, phone); ok {
		t.Error("a revoked token still works")
	}
	if err := s.RevokeAppToken(ctx, "alice", "phone"); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoking twice: %v, want ErrNotFound", err)
	}

	// Only the hash is kept.
	s.Close()
	server.Close()
	for _, f := range []string{path, path + "-wal"} {
		b, _ := os.ReadFile(f)
		if bytes.Contains(b, []byte(strings.TrimPrefix(phone, TokenPrefix))) {
			t.Errorf("%s holds a token in plain text", filepath.Base(f))
		}
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
			t.Errorf("database mode = %v, want 0600", fi.Mode().Perm())
		}
	}
}

// TestDirectoryIsPrivate: the -wal file SQLite writes beside the database
// takes the umask's mode, so the directory has to keep others out, even one
// that already existed with a looser mode.
func TestDirectoryIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix permission bits")
	}
	dir := filepath.Join(t.TempDir(), "db")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := Open(filepath.Join(dir, "midgard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Errorf("database directory mode = %v, want 0700", fi.Mode().Perm())
	}
}

// TestAppTokensKeepOwnersApart is the barrier for tokens: one person's
// tokens are invisible to another, and a name one has, another may too.
func TestAppTokensKeepOwnersApart(t *testing.T) {
	s, _ := open(t)
	ctx := context.Background()

	alice, err := s.IssueAppToken(ctx, "alice", "phone")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := s.IssueAppToken(ctx, "bob", "phone")
	if err != nil {
		t.Fatalf("bob cannot have a token named like alice's: %v", err)
	}

	if tokens, _ := s.AppTokens(ctx, "bob"); len(tokens) != 1 || tokens[0].Name != "phone" {
		t.Errorf("bob's tokens = %+v, want only his phone", tokens)
	}
	if owner, _, _ := s.CheckAppToken(ctx, bob); owner != "bob" {
		t.Errorf("bob's token acts for %q", owner)
	}
	if err := s.RevokeAppToken(ctx, "mallory", "phone"); !errors.Is(err, ErrNotFound) {
		t.Errorf("mallory revoking a phone: %v, want ErrNotFound", err)
	}
	if err := s.RevokeAppToken(ctx, "bob", "phone"); err != nil {
		t.Fatal(err)
	}
	if owner, _, ok := s.CheckAppToken(ctx, alice); !ok || owner != "alice" {
		t.Error("bob revoking his phone revoked alice's")
	}
	if _, err := s.IssueAppToken(ctx, "", "phone"); err == nil {
		t.Error("a token was issued with no owner")
	}
}

func TestInvalidTokenNames(t *testing.T) {
	s, _ := open(t)
	for _, name := range []string{"", "has space", "tab\there", "new\nline", strings.Repeat("x", 65), "ümlaut"} {
		if _, err := s.IssueAppToken(context.Background(), "alice", name); err == nil {
			t.Errorf("IssueAppToken(%q) accepted an invalid name", name)
		}
	}
}

// TestMigrate: opening a database again leaves it as it is, and one written
// by a newer server is refused rather than misread.
func TestMigrate(t *testing.T) {
	s, path := open(t)
	if _, err := s.IssueAppToken(context.Background(), "alice", "phone"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	if tokens, _ := s.AppTokens(context.Background(), "alice"); len(tokens) != 1 {
		t.Fatalf("reopening lost data: %+v", tokens)
	}
	if _, err := s.db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s, err := Open(path); err == nil {
		s.Close()
		t.Fatal("a database from a newer schema was opened")
	}
}
