// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"changkun.de/x/midgard/internal/store"
	"github.com/gin-gonic/gin"
)

// withStore gives m a database of its own for one test.
func withStore(t *testing.T, m *Midgard) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "midgard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	m.store = s
	return s
}

func TestAppTokenLogin(t *testing.T) {
	resetBlocklist(t)
	m := testMidgard(t)
	s := withStore(t, m)
	ctx := context.Background()

	tok, err := s.IssueAppToken(ctx, testUser, testEmail, "phone")
	if err != nil {
		t.Fatal(err)
	}
	bearer := func(tok, remote string) int {
		req := httptest.NewRequest(http.MethodGet, "/midgard/api/v1/clipboard", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		req.RemoteAddr = remote
		w := httptest.NewRecorder()
		m.routers().ServeHTTP(w, req)
		return w.Code
	}

	if code := bearer(tok, "192.0.2.1:1"); code != http.StatusOK {
		t.Fatalf("an issued token got %d, want 200", code)
	}
	// signing in keeps working alongside
	if w := do(t, m, http.MethodGet, "/midgard/api/v1/clipboard", "", true); w.Code != http.StatusOK {
		t.Fatalf("a signed-in request got %d, want 200", w.Code)
	}
	// and there is no password any more
	req := httptest.NewRequest(http.MethodGet, "/midgard/api/v1/clipboard", nil)
	req.SetBasicAuth("midgard", "password")
	req.RemoteAddr = "192.0.2.9:1"
	w := httptest.NewRecorder()
	m.routers().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("basic auth got %d, want 401: the password is gone", w.Code)
	}

	if err := s.RevokeAppToken(ctx, testUser, "phone"); err != nil {
		t.Fatal(err)
	}
	if code := bearer(tok, "192.0.2.1:1"); code != http.StatusUnauthorized {
		t.Fatalf("a revoked token got %d, want 401", code)
	}

	// guessing tokens is held to the same limit as guessing passwords
	for range maxFailureAttempts + 1 {
		bearer(store.TokenPrefix+"guess", "192.0.2.2:1")
	}
	if code := bearer(store.TokenPrefix+"guess", "192.0.2.2:1"); code != http.StatusForbidden {
		t.Fatalf("after repeated bad tokens got %d, want 403", code)
	}
}

// TestLoginNamesTheOwner: what a request may reach is decided by the owner
// the login sets, so it must be the token's owner, whatever else is sent.
func TestLoginNamesTheOwner(t *testing.T) {
	resetBlocklist(t)
	s, err := store.Open(filepath.Join(t.TempDir(), "midgard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tok, _ := s.IssueAppToken(context.Background(), testUser, testEmail, "phone")

	r := gin.New()
	r.Use(signIn(s, newLatereAuth(), nil))
	var owner, device string
	r.GET("/", func(c *gin.Context) { owner, device = c.GetString(ctxOwner), c.GetString(ctxDevice) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.RemoteAddr = "192.0.2.3:1"
	r.ServeHTTP(httptest.NewRecorder(), req)
	if owner != testUser || device != "phone" {
		t.Fatalf("owner, device = %q, %q; want %s, phone", owner, device, testUser)
	}
}
