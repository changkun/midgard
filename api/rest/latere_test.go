// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"latere.ai/x/pkg/authkit/issuertest"
)

// issuer stands in for auth.latere.ai, and allows alice by email and carol by
// principal id.
func issuer(t *testing.T) *issuertest.Server {
	t.Helper()
	srv := issuertest.New(t, issuertest.WithDefaultAudience(audience))
	t.Setenv("AUTH_URL", srv.URL())
	t.Setenv("AUTH_JWKS_URL", srv.JWKSURL())
	t.Setenv("AUTH_ALLOWED_PRINCIPALS", "Alice@Example.com, sub-carol")
	return srv
}

// signedIn serves one request through the login, and reports the status and
// the owner the login named.
func signedIn(t *testing.T, latere *latereAuth, tokens appTokens, header, remote string) (int, string) {
	t.Helper()
	r := gin.New()
	r.Use(BasicAuthWithAttemptsControl(Credentials{"midgard": "password"}, tokens, latere))
	var owner string
	r.GET("/", func(c *gin.Context) { owner = c.GetString(ctxOwner) })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", header)
	req.RemoteAddr = remote
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, owner
}

func TestLatereSignIn(t *testing.T) {
	resetBlocklist(t)
	srv := issuer(t)
	latere := newLatereAuth()
	other := issuertest.New(t, issuertest.WithDefaultAudience(audience))

	for i, tt := range []struct {
		name  string
		token string
		code  int
		owner string
	}{
		{"allowed by email", srv.Mint(issuertest.Claims{Sub: "sub-alice", Email: "alice@example.com"}), 200, "sub-alice"},
		{"allowed by principal id", srv.Mint(issuertest.Claims{Sub: "sub-carol"}), 200, "sub-carol"},
		{"not allowed", srv.Mint(issuertest.Claims{Sub: "sub-mallory", Email: "mallory@example.com"}), 401, ""},
		{"for another service", srv.Mint(issuertest.Claims{Sub: "sub-alice", Email: "alice@example.com", Aud: issuertest.StringList{"lux"}}), 401, ""},
		{"expired", srv.Mint(issuertest.Claims{Sub: "sub-alice", Email: "alice@example.com", Exp: time.Now().Add(-time.Hour).Unix()}), 401, ""},
		{"from another issuer", other.Mint(issuertest.Claims{Sub: "sub-alice", Email: "alice@example.com"}), 401, ""},
		{"not a token", "garbage", 401, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			code, owner := signedIn(t, latere, nil, "Bearer "+tt.token, "192.0.2."+string(rune('1'+i))+":1")
			if code != tt.code || owner != tt.owner {
				t.Fatalf("got %d as %q, want %d as %q", code, owner, tt.code, tt.owner)
			}
		})
	}
}

// TestAppTokensFollowTheAllowlist: with sign-in set up, an app token is only
// as good as its owner's place on the allowlist, so removing someone from it
// also stops the Shortcuts they made tokens for.
func TestAppTokensFollowTheAllowlist(t *testing.T) {
	resetBlocklist(t)
	issuer(t)
	s := withStore(t, NewMidgard())
	ctx := context.Background()
	alice, _ := s.IssueAppToken(ctx, "sub-alice", "alice@example.com", "phone")
	carol, _ := s.IssueAppToken(ctx, "sub-carol", "", "phone")
	mallory, _ := s.IssueAppToken(ctx, "sub-mallory", "", "phone")

	latere := newLatereAuth()
	for tok, want := range map[string]int{alice: 200, carol: 200, mallory: 401} {
		if code, _ := signedIn(t, latere, s, "Bearer "+tok, "192.0.2.20:1"); code != want {
			t.Errorf("app token: got %d, want %d", code, want)
		}
	}

	// alice leaves the allowlist
	t.Setenv("AUTH_ALLOWED_PRINCIPALS", "sub-carol")
	if code, _ := signedIn(t, newLatereAuth(), s, "Bearer "+alice, "192.0.2.21:1"); code != 401 {
		t.Errorf("an app token outlived its owner's place on the allowlist: got %d", code)
	}
}

// TestNoAllowlistNoSignIn: with no allowlist there is no safe answer but no,
// so latere tokens are refused rather than trusted wholesale.
func TestNoAllowlistNoSignIn(t *testing.T) {
	resetBlocklist(t)
	srv := issuer(t)
	t.Setenv("AUTH_ALLOWED_PRINCIPALS", "")
	if newLatereAuth() != nil {
		t.Fatal("sign-in was set up with no one allowed")
	}
	code, _ := signedIn(t, nil, nil, "Bearer "+srv.Mint(issuertest.Claims{Sub: "sub-alice"}), "192.0.2.30:1")
	if code != 401 {
		t.Fatalf("got %d, want 401", code)
	}
}
