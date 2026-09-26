// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package signin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/testdata"
	"golang.org/x/oauth2"
	"latere.ai/x/pkg/authkit/cli"
	"latere.ai/x/pkg/authkit/issuertest"
	"latere.ai/x/pkg/authkit/oidc"
)

func TestMain(m *testing.M) {
	testdata.UseConfig()
	os.Exit(m.Run())
}

// stub is auth.latere.ai for these tests: issuertest mints the tokens and
// serves /actor-tokens, and a refresh_token grant at /token is added here,
// which issuertest does not serve.
type stub struct {
	*issuertest.Server
	url       string
	refreshes atomic.Int32
}

func newStub(t *testing.T) *stub {
	t.Helper()
	s := &stub{}
	var issuer http.Handler
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/token" {
			r.ParseForm()
			if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "rt-1" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			s.refreshes.Add(1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": s.login(), "token_type": "Bearer", "expires_in": 3600,
				// no refresh_token: the old one stays in use
			})
			return
		}
		issuer.ServeHTTP(w, r)
	}))
	s.url = "http://" + srv.Listener.Addr().String()
	s.Server = issuertest.NewHandler(issuertest.WithIssuer(s.url))
	issuer = s.Handler()
	srv.Start()
	t.Cleanup(srv.Close)
	return s
}

// login is a sign-in token for alice, addressed to the issuer.
func (s *stub) login() string {
	return s.Mint(issuertest.Claims{Sub: "sub-alice", Email: "alice@example.com", Aud: issuertest.StringList{s.url}})
}

func (s *stub) source(t *testing.T, tok *oauth2.Token) (*source, cli.TokenStore) {
	t.Helper()
	store, err := cli.NewFileTokenStore(filepath.Join(t.TempDir(), "token.json"))
	if err != nil {
		t.Fatal(err)
	}
	if tok != nil {
		if err := store.Save(tok); err != nil {
			t.Fatal(err)
		}
	}
	c := oidc.New(oidc.Config{AuthURL: s.url, ClientID: "midgard-cli"})
	return &source{client: c, store: store}, store
}

// claims reads a JWT's payload, unverified: the server verifies, and these
// tests only need to see what was asked for.
func claims(t *testing.T, header string) map[string]any {
	t.Helper()
	tok, ok := strings.CutPrefix(header, "Bearer ")
	parts := strings.Split(tok, ".")
	if !ok || len(parts) != 3 {
		t.Fatalf("%q is not a bearer JWT", header)
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var c map[string]any
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func audiences(c map[string]any) []string {
	switch aud := c["aud"].(type) {
	case string:
		return []string{aud}
	case []any:
		var out []string
		for _, a := range aud {
			out = append(out, a.(string))
		}
		return out
	}
	return nil
}

func TestSignedOut(t *testing.T) {
	src, _ := newStub(t).source(t, nil)
	if _, err := src.authorization(context.Background()); !errors.Is(err, ErrSignedOut) {
		t.Fatalf("with no sign-in: %v, want ErrSignedOut", err)
	}
}

// TestCallsCarryAMidgardToken: the sign-in is addressed to auth.latere.ai and
// opens nothing else, so what a call sends is a token minted for midgard, for
// the same person, and minted once rather than per call.
func TestCallsCarryAMidgardToken(t *testing.T) {
	s := newStub(t)
	src, _ := s.source(t, &oauth2.Token{AccessToken: s.login(), RefreshToken: "rt-1", Expiry: time.Now().Add(time.Hour)})
	ctx := context.Background()

	h1, err := src.authorization(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := claims(t, h1)
	if !slices.Contains(audiences(c), audience) || c["sub"] != "sub-alice" {
		t.Fatalf("the call carries aud=%v sub=%v, want aud=%s sub=sub-alice", c["aud"], c["sub"], audience)
	}
	h2, _ := src.authorization(ctx)
	if h1 != h2 {
		t.Error("a second call minted another token instead of reusing the first")
	}
	if n := s.refreshes.Load(); n != 0 {
		t.Errorf("a fresh sign-in was refreshed %d times", n)
	}
}

// TestSignInIsRenewed: a daemon runs for days, far past its first access
// token; the sign-in is refreshed when it runs out, and the renewal kept.
func TestSignInIsRenewed(t *testing.T) {
	s := newStub(t)
	src, store := s.source(t, &oauth2.Token{AccessToken: s.login(), RefreshToken: "rt-1", Expiry: time.Now().Add(-time.Minute)})

	h, err := src.authorization(context.Background())
	if err != nil {
		t.Fatalf("an expired sign-in with a refresh token: %v", err)
	}
	if !slices.Contains(audiences(claims(t, h)), audience) {
		t.Fatal("the renewed sign-in did not yield a midgard token")
	}
	if n := s.refreshes.Load(); n != 1 {
		t.Fatalf("refreshed %d times, want 1", n)
	}
	saved, _ := store.Load()
	if !saved.Expiry.After(time.Now()) || saved.RefreshToken != "rt-1" {
		t.Fatalf("saved %+v: want the renewed token, keeping the refresh token", saved)
	}
}

func TestExpiredWithoutRefresh(t *testing.T) {
	s := newStub(t)
	src, _ := s.source(t, &oauth2.Token{AccessToken: s.login(), Expiry: time.Now().Add(-time.Minute)})
	if _, err := src.authorization(context.Background()); !errors.Is(err, ErrSignedOut) {
		t.Fatalf("an expired sign-in with no refresh token: %v, want ErrSignedOut", err)
	}
}

// TestAppTokenFirst: a device given an app token uses it and needs no sign-in.
func TestAppTokenFirst(t *testing.T) {
	saved := config.Get().Token
	t.Cleanup(func() { config.Get().Token = saved })
	config.Get().Token = "mgt_device"
	if h, err := Authorization(context.Background()); err != nil || h != "Bearer mgt_device" {
		t.Fatalf("Authorization() = %q, %v; want the app token", h, err)
	}
}
