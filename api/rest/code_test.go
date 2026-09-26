// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
)

// TestCodeListingWithoutStore: with backups off, the store has no code
// directory, and the listing dereferenced the nil entry WalkDir reports for
// it, so every visit to /midgard/code panicked.
func TestCodeListingWithoutStore(t *testing.T) {
	t.Chdir(t.TempDir())
	if w := do(t, NewMidgard(), http.MethodGet, "/midgard/code", "", false); w.Code != http.StatusOK {
		t.Fatalf("got %d (%s), want 200", w.Code, w.Body)
	}
}

// TestCode2imgWithoutStore: code2img wrote into a code directory that only the
// backup creates, so with backups off it always failed.
func TestCode2imgWithoutStore(t *testing.T) {
	if !findChrome() {
		t.Skip("code2img needs Chrome or Chromium")
	}
	chromeFound = true
	t.Cleanup(func() { chromeFound = false })
	t.Chdir(t.TempDir())

	w := do(t, NewMidgard(), http.MethodPost, "/midgard/api/v1/code2img", `{"code":"package main"}`, true)
	var out types.Code2ImgOutput
	json.Unmarshal(w.Body.Bytes(), &out)

	// The code is saved before it is rendered, so it must be there even
	// where Chrome cannot render.
	codes, _ := filepath.Glob(filepath.Join(config.RepoPath, "code", "*"))
	if len(codes) == 0 {
		t.Fatalf("the code was not saved: got %d (%s)", w.Code, out.Message)
	}
	// Rendering needs a working Chrome, which CI runners do not reliably
	// have: on Ubuntu it may be denied its sandbox, or start without ever
	// answering. What this test is about, saving, is checked above.
	if strings.HasPrefix(out.Message, "failed to render code image") {
		t.Skipf("the code was saved, but Chrome could not render it here: %s", out.Message)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("got %d (%s), want 200", w.Code, out.Message)
	}
	for _, u := range []string{out.Code, out.Image} {
		name := filepath.Join(config.RepoPath, filepath.FromSlash(u[len(config.S().Store.Prefix):]))
		if _, err := os.Stat(name); err != nil {
			t.Errorf("%s was not saved: %v", u, err)
		}
	}
}
