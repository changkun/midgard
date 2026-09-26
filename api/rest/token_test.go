// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/token"
)

func TestDeviceTokens(t *testing.T) {
	t.Chdir(t.TempDir())
	resetBlocklist(t)
	m := NewMidgard()

	tok, err := token.Open(config.TokensPath).Add("phone")
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
	// the password keeps working alongside, for Shortcuts and Tasker
	if w := do(t, m, http.MethodGet, "/midgard/api/v1/clipboard", "", true); w.Code != http.StatusOK {
		t.Fatalf("basic auth got %d, want 200", w.Code)
	}

	if err := token.Open(config.TokensPath).Remove("phone"); err != nil {
		t.Fatal(err)
	}
	if code := bearer(tok, "192.0.2.1:1"); code != http.StatusUnauthorized {
		t.Fatalf("a revoked token got %d, want 401", code)
	}

	// guessing tokens is held to the same limit as guessing passwords
	for range maxFailureAttempts + 1 {
		bearer(token.Prefix+"guess", "192.0.2.2:1")
	}
	if code := bearer(token.Prefix+"guess", "192.0.2.2:1"); code != http.StatusForbidden {
		t.Fatalf("after repeated bad tokens got %d, want 403", code)
	}
}
