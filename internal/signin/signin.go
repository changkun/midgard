// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package signin is how a device signs in to midgard through auth.latere.ai
// (specs/redesign.md §4).
//
// mg login runs the device grant once: it prints a code, the person approves
// it in a browser, and the sign-in is kept in the user's configuration
// directory. That sign-in is addressed to auth.latere.ai and opens nothing
// else, so every call to the server carries a token minted from it for
// midgard alone, short-lived and reused until it nears expiry. The sign-in
// itself is refreshed as it runs out, so a daemon keeps working for as long
// as the refresh token does.
//
// A device that cannot sign in that way can be given an app token instead, set
// as token: in its config.yml.
package signin

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"changkun.de/x/midgard/internal/config"
	"latere.ai/x/pkg/authkit/cli"
	"latere.ai/x/pkg/authkit/oidc"
)

// The client midgard's devices sign in as, and the audience their calls are
// minted for. Both are registered with auth.latere.ai.
const (
	defaultAuthURL  = "https://auth.latere.ai"
	defaultClientID = "midgard-cli"
	audience        = "midgard"
)

// ErrSignedOut means this device has no sign-in and no app token.
var ErrSignedOut = errors.New("this device is not signed in; run mg login")

// authURL and clientID are auth.latere.ai and midgard's client there, unless
// AUTH_URL or AUTH_CLIENT_ID say otherwise.
func authURL() string {
	return strings.TrimRight(cmp.Or(os.Getenv("AUTH_URL"), defaultAuthURL), "/")
}

func clientID() string { return cmp.Or(os.Getenv("AUTH_CLIENT_ID"), defaultClientID) }

// tokenPath is where the sign-in is kept: midgard's own, not the file other
// latere tools share, since a refresh in one would strand the other.
func tokenPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "midgard", "token.json"), nil
}

func newClient() *oidc.Client {
	return oidc.New(oidc.Config{
		AuthURL:  authURL(),
		ClientID: clientID(),
		// offline_access for a refresh token, without which a daemon would
		// be signed out when the first access token expired.
		Scopes: []string{"openid", "email", "profile", "offline_access"},
	})
}

func newStore() (*cli.FileTokenStore, error) {
	path, err := tokenPath()
	if err != nil {
		return nil, err
	}
	return cli.NewFileTokenStore(path)
}

// Login signs this device in with the device grant, and keeps the sign-in.
func Login(ctx context.Context) error {
	store, err := newStore()
	if err != nil {
		return err
	}
	return cli.NewDeviceCodeClient(newClient(), store).Login(ctx)
}

// Logout forgets this device's sign-in.
func Logout() error {
	store, err := newStore()
	if err != nil {
		return err
	}
	return store.Clear()
}

// source keeps one process's sign-in fresh. Its client caches the tokens
// minted for midgard.
type source struct {
	mu     sync.Mutex
	client *oidc.Client
	store  cli.TokenStore
}

var shared = sync.OnceValues(func() (*source, error) {
	store, err := newStore()
	if err != nil {
		return nil, err
	}
	return &source{client: newClient(), store: store}, nil
})

// Authorization is the Authorization header for a call to the midgard server:
// this device's app token when config.yml sets one, and otherwise a token
// minted for midgard from its sign-in.
func Authorization(ctx context.Context) (string, error) {
	if t := config.Get().Token; t != "" {
		return "Bearer " + t, nil
	}
	s, err := shared()
	if err != nil {
		return "", err
	}
	return s.authorization(ctx)
}

func (s *source) authorization(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tok, err := s.store.Load()
	if err != nil {
		return "", err
	}
	if tok == nil || tok.AccessToken == "" {
		return "", ErrSignedOut
	}
	// Refresh a little early, so the token minted from it does not outlive
	// the sign-in it came from mid-request.
	if time.Until(tok.Expiry) < time.Minute && !tok.Expiry.IsZero() {
		if tok.RefreshToken == "" {
			return "", fmt.Errorf("the sign-in expired; run mg login: %w", ErrSignedOut)
		}
		fresh, err := s.client.RefreshTokenContext(ctx, tok.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("cannot renew the sign-in (run mg login): %w", err)
		}
		if fresh.RefreshToken == "" { // a refresh may reuse the old one
			fresh.RefreshToken = tok.RefreshToken
		}
		if err := s.store.Save(fresh); err != nil {
			return "", err
		}
		tok = fresh
	}

	actor, _, err := s.client.ActorToken(ctx, &oidc.Session{AccessToken: tok.AccessToken}, audience)
	if err != nil {
		return "", fmt.Errorf("cannot get a token for midgard: %w", err)
	}
	return "Bearer " + actor, nil
}
