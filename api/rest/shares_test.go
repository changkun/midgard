// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
)

// share asks m to publish in, as testUser.
func share(t *testing.T, m *Midgard, in types.ShareInput) (int, types.ShareInfo) {
	t.Helper()
	b, _ := json.Marshal(in)
	w := do(t, m, http.MethodPost, "/midgard/api/v1/shares", string(b), true)
	var out types.ShareInfo
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func TestShare(t *testing.T) {
	m := testMidgard(t)

	code, named := share(t, m, types.ShareInput{Data: b64("hello"), Type: "text/plain", Name: "/notes/hello.txt"})
	if code != http.StatusOK || named.URL != "/midgard/notes/hello.txt" || named.Name != "notes/hello.txt" || named.Size != 5 {
		t.Fatalf("a named share: %d %+v", code, named)
	}
	code, random := share(t, m, types.ShareInput{Data: b64("<script>alert(1)</script>"), Type: "text/html"})
	if code != http.StatusOK || random.URL != "/midgard/s/"+random.Slug || random.Expires != nil {
		t.Fatalf("a share without a name: %d %+v", code, random)
	}

	// the links are public: no sign-in
	for _, tt := range []struct{ path, body, typ string }{
		{named.URL, "hello", "text/plain; charset=utf-8"},
		{"/midgard/s/" + named.Slug, "hello", "text/plain; charset=utf-8"},
		{named.URL + "?download=1", "hello", "text/plain; charset=utf-8"},
		{random.URL, "<script>alert(1)</script>", "text/html"},
	} {
		w := do(t, m, http.MethodGet, tt.path, "", false)
		if w.Code != http.StatusOK || w.Body.String() != tt.body || w.Header().Get("Content-Type") != tt.typ {
			t.Errorf("GET %s: %d %q %q, want %q as %q", tt.path, w.Code, w.Header().Get("Content-Type"), w.Body, tt.body, tt.typ)
		}
		// Shares are served from midgard's own origin; a shared page must
		// not run as it.
		if got := w.Header().Get("Content-Security-Policy"); got != "sandbox" {
			t.Errorf("GET %s: Content-Security-Policy = %q, want sandbox", tt.path, got)
		}
		if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("GET %s: X-Content-Type-Options = %q, want nosniff", tt.path, got)
		}
	}
	for _, path := range []string{"/midgard/s/nosuchslug", "/midgard/notes/nosuch.txt", "/midgard/s/" + named.Slug + "/x", "/elsewhere/notes/hello.txt"} {
		if w := do(t, m, http.MethodGet, path, "", false); w.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", path, w.Code)
		}
	}
	// a link is only to be read
	if w := do(t, m, http.MethodPost, named.URL, "x", false); w.Code != http.StatusNotFound {
		t.Errorf("POST %s: %d, want 404", named.URL, w.Code)
	}

	if code, _ := share(t, m, types.ShareInput{Data: b64("again"), Name: "notes/hello.txt"}); code != http.StatusConflict {
		t.Errorf("a taken name: %d, want 409", code)
	}
	if w := do(t, m, http.MethodGet, named.URL, "", false); w.Body.String() != "hello" {
		t.Errorf("a taken name was overwritten: %q", w.Body)
	}
	for _, in := range []types.ShareInput{{Data: "not base64!"}, {Data: b64("\n")}} {
		if code, _ := share(t, m, in); code != http.StatusBadRequest {
			t.Errorf("share %+v: %d, want 400", in, code)
		}
	}
	// signed out, nothing may be shared
	if w := do(t, m, http.MethodPost, "/midgard/api/v1/shares", `{"data":"eA=="}`, false); w.Code != http.StatusUnauthorized {
		t.Errorf("sharing signed out: %d, want 401", w.Code)
	}
}

// TestShareNames is what a name may be: part of a URL under the prefix, never
// a way out of it, onto a hidden file, or over midgard's own routes.
func TestShareNames(t *testing.T) {
	m := testMidgard(t)
	for _, name := range []string{
		"../escape.txt",
		"/../../escape.txt",
		"notes/../../escape.txt",
		"..",
		"/",
		".git/config",
		"notes/.hidden",
		"api/v1/clipboard",
		"s/abc",
		"ping",
	} {
		t.Run(name, func(t *testing.T) {
			if code, out := share(t, m, types.ShareInput{Data: b64("x"), Name: name}); code != http.StatusBadRequest {
				t.Fatalf("got %d %+v, want 400", code, out)
			}
		})
	}
	// a name is cleaned to the one it means
	if code, out := share(t, m, types.ShareInput{Data: b64("x"), Name: "notes//./a.txt"}); code != http.StatusOK || out.Name != "notes/a.txt" {
		t.Fatalf("notes//./a.txt: %d %+v, want notes/a.txt", code, out)
	}
}

// TestShareTheClipboard: with no data, a share is of the requester's own
// clipboard.
func TestShareTheClipboard(t *testing.T) {
	m := testMidgard(t)
	if code, _ := share(t, m, types.ShareInput{}); code != http.StatusBadRequest {
		t.Fatalf("sharing an empty clipboard: %d, want 400", code)
	}
	if w := do(t, m, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"copied"}`, true); w.Code != http.StatusOK {
		t.Fatalf("copy: %d %s", w.Code, w.Body)
	}
	code, out := share(t, m, types.ShareInput{ExpiresIn: 3600})
	if code != http.StatusOK || out.Type != types.MIMEPlainText || out.Expires == nil || time.Until(*out.Expires) < 59*time.Minute {
		t.Fatalf("sharing the clipboard: %d %+v", code, out)
	}
	if w := do(t, m, http.MethodGet, out.URL, "", false); w.Body.String() != "copied" {
		t.Fatalf("the share holds %q, want the clipboard", w.Body)
	}
}

func TestShareExpires(t *testing.T) {
	m := testMidgard(t)
	sh, err := m.store.CreateShare(context.Background(), testUser, "gone", "text", []byte("x"), time.Now().Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/midgard/gone", "/midgard/s/" + sh.Slug} {
		if w := do(t, m, http.MethodGet, path, "", false); w.Code != http.StatusNotFound {
			t.Errorf("GET %s after it expired: %d, want 404", path, w.Code)
		}
	}
}

// TestSharesAreApart is the barrier for shares: anyone may open a link, but
// only its owner lists or revokes it.
func TestSharesAreApart(t *testing.T) {
	resetBlocklist(t)
	t.Setenv("AUTH_ALLOWED_PRINCIPALS", "alice@example.com, bob@example.com")
	m := testMidgard(t)
	srv := httptest.NewServer(m.routers())
	t.Cleanup(srv.Close)
	alice := person{"sub-alice", "alice@example.com"}
	bob := person{"sub-bob", "bob@example.com"}

	code, body := alice.request(t, srv, http.MethodPost, "/midgard/api/v1/shares", `{"data":"`+b64("alice's")+`","type":"text"}`)
	var sh types.ShareInfo
	if json.Unmarshal(body, &sh); code != http.StatusOK {
		t.Fatalf("alice shares: %d %s", code, body)
	}

	_, body = bob.request(t, srv, http.MethodGet, "/midgard/api/v1/shares", "")
	if strings.Contains(string(body), sh.Slug) {
		t.Fatalf("bob lists alice's share: %s", body)
	}
	if code, _ := bob.request(t, srv, http.MethodDelete, "/midgard/api/v1/shares/"+sh.Slug, ""); code != http.StatusNotFound {
		t.Fatalf("bob revokes alice's share: %d, want 404", code)
	}
	res, err := http.Get(srv.URL + sh.URL)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("after bob's attempt, alice's link: %v %v", res.Status, err)
	}
	res.Body.Close()

	_, body = alice.request(t, srv, http.MethodGet, "/midgard/api/v1/shares", "")
	var list types.SharesOutput
	if json.Unmarshal(body, &list); len(list.Shares) != 1 || list.Shares[0].Slug != sh.Slug {
		t.Fatalf("alice's shares: %s", body)
	}
	if code, _ := alice.request(t, srv, http.MethodDelete, "/midgard/api/v1/shares/"+sh.Slug, ""); code != http.StatusNoContent {
		t.Fatalf("alice revokes her share: %d, want 204", code)
	}
	res, err = http.Get(srv.URL + sh.URL)
	if err != nil || res.StatusCode != http.StatusNotFound {
		t.Fatalf("a revoked link: %v %v, want 404", res.Status, err)
	}
	res.Body.Close()
}

// TestFilesAreNotServed: shares are served from the database. A store left on
// disk by an older server, its old git backup's .git included, is not served
// at all, until it is imported.
func TestFilesAreNotServed(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, name := range []string{"notes/a.txt", ".git/config"} {
		p := filepath.Join(config.RepoPath, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("on disk"), 0o644)
	}
	m := testMidgard(t)
	for _, path := range []string{"/midgard/notes/a.txt", "/midgard/.git/config", "/midgard/%2egit/config"} {
		if w := do(t, m, http.MethodGet, path, "", false); w.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d %q, want 404", path, w.Code, w.Body)
		}
	}
}
