// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/store"
)

func TestImportShares(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	put := func(name, body string) {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("random/fboVP8u4xNMHfvsv2EeLzL.txt", "a copy")
	put("img/shot.png", "\x89PNG\r\n\x1a\nnot really")
	put("awesome/filename", "<html>a page</html>")
	put("empty", "")
	put(".git/config", "[remote]")
	put("notes/.private", "never served")
	put("api/v1/x", "under midgard's own routes")
	put("taken.txt", "the old one")
	old := time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC)
	os.Chtimes(filepath.Join(dir, "img", "shot.png"), old, old)
	if runtime.GOOS != "windows" {
		os.Symlink("/etc/passwd", filepath.Join(dir, "link"))
	}

	s, err := store.Open(filepath.Join(t.TempDir(), "midgard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// someone shared at that name since the new server went up
	if _, err := s.CreateShare(ctx, "bob", "taken.txt", "text", []byte("bob's"), time.Time{}); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	r, err := importShares(ctx, s, dir, "alice", true, &out)
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Shares(ctx, "alice"); len(list) != 0 {
		t.Fatalf("a dry run imported %d shares", len(list))
	}
	wantFailed := 2 // api/v1/x and taken.txt
	if runtime.GOOS != "windows" {
		wantFailed++ // the link
	}
	if r.imported != 4 || r.hidden != 2 || r.failed != wantFailed {
		t.Fatalf("dry run: %+v\n%s", r, &out)
	}

	out.Reset()
	r, err = importShares(ctx, s, dir, "alice", false, &out)
	if err != nil {
		t.Fatal(err)
	}
	if r.imported != 4 || r.present != 0 || r.hidden != 2 || r.failed != wantFailed {
		t.Fatalf("import: %+v\n%s", r, &out)
	}
	for _, want := range []string{"api/v1/x", "taken.txt: the name is taken", "left behind .git: hidden", "left behind notes/.private: hidden"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report does not say %q:\n%s", want, &out)
		}
	}

	// every old link keeps working, as what it was served as
	for name, want := range map[string]struct{ body, mime string }{
		"random/fboVP8u4xNMHfvsv2EeLzL.txt": {"a copy", "text/plain; charset=utf-8"},
		"img/shot.png":                      {"\x89PNG\r\n\x1a\nnot really", "image/png"},
		"awesome/filename":                  {"<html>a page</html>", "text/html; charset=utf-8"},
		"empty":                             {"", "text/plain; charset=utf-8"},
	} {
		sh, err := s.ShareByPath(ctx, name)
		if err != nil || sh.Owner != "alice" || string(sh.Data) != want.body || sh.MIME != want.mime || !sh.Expires.IsZero() {
			t.Errorf("%s: %+v, %v; want alice's %q as %s", name, sh, err, want.body, want.mime)
		}
	}
	if sh, _ := s.ShareByPath(ctx, "img/shot.png"); !sh.Created.Equal(old) {
		t.Errorf("img/shot.png was shared %v, want when the file was made, %v", sh.Created, old)
	}
	if sh, _ := s.ShareByPath(ctx, "taken.txt"); sh.Owner != "bob" {
		t.Errorf("the import took bob's name: %+v", sh)
	}
	for _, name := range []string{".git/config", "notes/.private", "link"} {
		if _, err := s.ShareByPath(ctx, name); err == nil {
			t.Errorf("%s was imported", name)
		}
	}

	// again: nothing changes
	out.Reset()
	r, err = importShares(ctx, s, dir, "alice", false, &out)
	if err != nil {
		t.Fatal(err)
	}
	if r.imported != 0 || r.present != 4 {
		t.Fatalf("a second import: %+v\n%s", r, &out)
	}
	if list, _ := s.Shares(ctx, "alice"); len(list) != 4 {
		t.Fatalf("after two imports alice has %d shares, want 4", len(list))
	}
}
