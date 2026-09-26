// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"changkun.de/x/midgard/internal/config"
)

// TestStoreHidesHiddenFiles covers a store left by the old git backup: its .git
// used to be served like any published file.
func TestStoreHidesHiddenFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	for name, body := range map[string]string{
		"notes/a.txt":    "published",
		".git/config":    "[remote]",
		".git/HEAD":      "ref: refs/heads/main",
		"notes/.private": "not published",
	} {
		p := filepath.Join(config.RepoPath, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := testMidgard(t)

	for _, tt := range []struct {
		path string
		want int
	}{
		{"/midgard/notes/a.txt", http.StatusOK},
		// a query used to make every published file 404
		{"/midgard/notes/a.txt?download=1", http.StatusOK},
		{"/midgard/.git/config", http.StatusNotFound},
		{"/midgard/.git/HEAD", http.StatusNotFound},
		{"/midgard/%2egit/config", http.StatusNotFound},
		{"/midgard/notes/.private", http.StatusNotFound},
		{"/midgard/notes/../.git/config", http.StatusNotFound},
	} {
		t.Run(tt.path, func(t *testing.T) {
			w := do(t, m, http.MethodGet, tt.path, "", false)
			if w.Code != tt.want {
				t.Fatalf("got %d (%q), want %d", w.Code, w.Body.String(), tt.want)
			}
			if tt.want == http.StatusOK && w.Body.String() != "published" {
				t.Fatalf("got body %q, want the published file", w.Body.String())
			}
		})
	}
}
