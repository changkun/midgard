// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
)

// allocate asks the server to publish data under uri.
func allocate(t *testing.T, m *Midgard, uri string, data []byte) (int, types.AllocateURLOutput) {
	t.Helper()
	in, err := json.Marshal(types.AllocateURLInput{
		Source: types.SourceAttachment,
		URI:    uri,
		Data:   base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		t.Fatal(err)
	}
	w := do(t, m, http.MethodPut, "/midgard/api/v1/allocate", string(in), true)
	var out types.AllocateURLOutput
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("allocate response is not valid json: %v (%s)", err, w.Body)
	}
	return w.Code, out
}

func TestAllocate(t *testing.T) {
	t.Chdir(t.TempDir())
	m := testMidgard(t)

	code, out := allocate(t, m, "/notes/hello.txt", []byte("hi"))
	if code != http.StatusOK || out.URL != "/midgard/notes/hello.txt" {
		t.Fatalf("allocate: got %d %+v, want 200 and /midgard/notes/hello.txt", code, out)
	}
	fi, err := os.Stat(filepath.Join(config.RepoPath, "notes", "hello.txt"))
	if err != nil {
		t.Fatalf("the allocated file is missing: %v", err)
	}
	// Published files are readable by everyone and writable by the server
	// only; they used to be created 0777.
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o644 {
		t.Errorf("allocated file mode = %v, want 0644", fi.Mode().Perm())
	}

	if code, _ := allocate(t, m, "notes/hello.txt", []byte("again")); code != http.StatusBadRequest {
		t.Errorf("allocating a taken uri: got %d, want 400", code)
	}
	if b, _ := os.ReadFile(filepath.Join(config.RepoPath, "notes", "hello.txt")); string(b) != "hi" {
		t.Errorf("a taken uri was overwritten: %q", b)
	}
}

// TestAllocateStaysInStore is the path traversal: the URI comes from the
// client, and it used to be joined onto the store path as it was.
func TestAllocateStaysInStore(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	m := testMidgard(t)

	for _, uri := range []string{
		"../escape.txt",
		"/../../escape.txt",
		"notes/../../escape.txt",
		"..",
		"/",
		".git/config",
		"notes/.hidden",
	} {
		t.Run(uri, func(t *testing.T) {
			if code, out := allocate(t, m, uri, []byte("x")); code != http.StatusBadRequest {
				t.Fatalf("got %d %+v, want 400", code, out)
			}
		})
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*", "escape.txt"))
	if _, err := os.Stat(filepath.Join(dir, "escape.txt")); err == nil || len(matches) > 0 {
		t.Fatal("a file was written outside the store")
	}
}

// TestAllocateThroughSymlink covers what a name check cannot: a link already
// inside the store that points out of it.
func TestAllocateThroughSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symbolic links needs extra privileges on Windows")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	outside := t.TempDir()
	if err := os.MkdirAll(config.RepoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(config.RepoPath, "link")); err != nil {
		t.Fatal(err)
	}

	code, _ := allocate(t, testMidgard(t), "link/escape.txt", []byte("x"))
	if code == http.StatusOK {
		t.Errorf("allocating through a symlink out of the store succeeded")
	}
	if _, err := os.Stat(filepath.Join(outside, "escape.txt")); err == nil {
		t.Fatal("a file was written through a symlink outside the store")
	}
}
