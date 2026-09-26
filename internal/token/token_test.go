// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package token

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "tokens")
	s := Open(path)

	laptop, err := s.Add("laptop")
	if err != nil {
		t.Fatal(err)
	}
	phone, err := s.Add("phone")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(laptop, Prefix) || laptop == phone {
		t.Fatalf("tokens %q and %q: want distinct, starting with %q", laptop, phone, Prefix)
	}

	// A second store on the same file is the running server, which must
	// see what mg server token wrote.
	server := Open(path)
	if name, ok := server.Check(laptop); !ok || name != "laptop" {
		t.Fatalf("Check(laptop) = %q, %v", name, ok)
	}
	for _, bad := range []string{"", Prefix, Prefix + "nope", laptop + "x", strings.TrimPrefix(laptop, Prefix)} {
		if name, ok := server.Check(bad); ok {
			t.Errorf("Check(%q) accepted it as %q", bad, name)
		}
	}

	if err := s.Remove("laptop"); err != nil {
		t.Fatal(err)
	}
	if _, ok := server.Check(laptop); ok {
		t.Error("a revoked token is still accepted")
	}
	if _, ok := server.Check(phone); !ok {
		t.Error("revoking one token revoked another")
	}

	infos, err := server.List()
	if err != nil || len(infos) != 1 || infos[0].Name != "phone" {
		t.Fatalf("List() = %+v, %v; want only phone", infos, err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), phone) || strings.Contains(string(b), strings.TrimPrefix(phone, Prefix)) {
		t.Error("the token file holds a token, not only its hash")
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
			t.Errorf("token file mode = %v, want 0600", fi.Mode().Perm())
		}
	}
}

func TestStoreRejects(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "tokens"))
	if _, err := s.Add("laptop"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("laptop"); err == nil {
		t.Error("a second token under the same name was issued")
	}
	for _, name := range []string{"", "has space", "tab\there", "new\nline", strings.Repeat("x", 65), "ümlaut"} {
		if _, err := s.Add(name); err == nil {
			t.Errorf("Add(%q) accepted an invalid name", name)
		}
	}
	if err := s.Remove("nobody"); err == nil {
		t.Error("removing a name without a token succeeded")
	}
}

func TestMissingFileIsEmpty(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "absent"))
	if _, ok := s.Check(Prefix + "anything"); ok {
		t.Error("a token was accepted with no token file")
	}
	if infos, err := s.List(); err != nil || len(infos) != 0 {
		t.Errorf("List() = %v, %v; want none", infos, err)
	}
}
