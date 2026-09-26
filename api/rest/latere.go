// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"cmp"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"latere.ai/x/pkg/authkit/jwt"
)

// audience is what a token must be minted for to open midgard. A token from
// auth.latere.ai for another service carries the same signature; its
// audience is the only thing that refuses it.
const audience = "midgard"

// latereAuth accepts the tokens auth.latere.ai issues for midgard: mg mints
// one per call from the person's sign-in (specs/redesign.md §4).
//
// A valid signature proves who is calling, not that they may use this
// server: anyone with a latere account can mint a token for any audience the
// registry lets their client name. The allowlist decides who may, as it does
// for changkun.de's other services.
type latereAuth struct {
	auth    *jwt.Authenticator
	allowed map[string]bool // lowercased email or principal id (sub)
}

// newLatereAuth builds the verifier from the environment: AUTH_URL (the
// issuer, https://auth.latere.ai by default), AUTH_JWKS_URL, and
// AUTH_ALLOWED_PRINCIPALS, a comma-separated list of emails and principal
// ids. It returns nil when the allowlist is empty, and then no latere token
// is accepted: with no one allowed there is no safe answer but no.
func newLatereAuth() *latereAuth {
	allowed := principalSet(os.Getenv("AUTH_ALLOWED_PRINCIPALS"))
	if len(allowed) == 0 {
		return nil
	}
	issuer := strings.TrimRight(cmp.Or(os.Getenv("AUTH_URL"), "https://auth.latere.ai"), "/")
	jwks := cmp.Or(os.Getenv("AUTH_JWKS_URL"), issuer+"/.well-known/jwks.json")
	slog.Info("latere sign-in enabled", "issuer", issuer, "principals", len(allowed))
	return &latereAuth{
		auth: jwt.NewAuthenticator(jwt.New(jwt.Config{
			JWKSURL:   jwks,
			Issuer:    issuer,
			Audiences: []string{audience},
		})),
		allowed: allowed,
	}
}

// principalSet parses a comma-separated list of emails and principal ids.
func principalSet(s string) map[string]bool {
	set := map[string]bool{}
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			set[p] = true
		}
	}
	return set
}

// identify reports the principal a latere token in r belongs to, when it is
// one for midgard from someone the allowlist admits. The owner of everything
// they store is the principal id, which, unlike an email, never changes.
func (a *latereAuth) identify(r *http.Request) (owner string, ok bool) {
	if a == nil {
		return "", false
	}
	id, err := a.auth.Authenticate(r)
	if err != nil || id.Sub == "" {
		return "", false
	}
	if !a.permits(id.Sub, id.Email) {
		slog.Warn("latere principal not allowed", "sub", id.Sub, "email", id.Email)
		return "", false
	}
	return id.Sub, true
}

// permits reports whether the allowlist admits the principal sub, known by
// email. A nil receiver admits no one.
func (a *latereAuth) permits(sub, email string) bool {
	if a == nil {
		return false
	}
	return a.allowed[strings.ToLower(sub)] || (email != "" && a.allowed[strings.ToLower(email)])
}
