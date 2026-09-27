// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"changkun.de/x/midgard/internal/store"

	"latere.ai/x/pkg/authkit"
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
	emails  *principals     // who signed in on the web page, by email; nil keeps none
}

// errNotOnList is a good sign-in the allowlist does not admit: it is
// answered 403 with types.MsgNotOnList, and not counted against the
// address, as a guess at a token is.
var errNotOnList = errors.New("not on the allowlist")

// principals remembers the email auth.latere.ai vouched for each principal
// who signed in on the web page. A device's token names its principal alone
// (auth.latere.ai's actor tokens carry no email), so an allowlist that
// names someone by email admits their devices once they have signed in on
// the page, from any of its sessions, allowed or turned away.
type principals struct {
	store func() *store.Store // read when asked: a test sets it late
	mu    sync.Mutex
	known map[string]string // sub → email, as last learned or read
}

// learn records that sub is known by email.
func (p *principals) learn(sub, email string) {
	if p == nil || sub == "" || email == "" {
		return
	}
	email = strings.ToLower(email)
	p.mu.Lock()
	same := p.known[sub] == email
	if p.known == nil {
		p.known = map[string]string{}
	}
	p.known[sub] = email
	p.mu.Unlock()
	if s := p.store(); s != nil && !same {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.LearnPrincipal(ctx, sub, email); err != nil {
			slog.Error("cannot remember a principal's email", "sub", sub, "err", err)
		}
	}
}

// emailOf is the email sub is known by, "" if none.
func (p *principals) emailOf(sub string) string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	email, ok := p.known[sub]
	p.mu.Unlock()
	if ok {
		return email
	}
	s := p.store()
	if s == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	email, err := s.PrincipalEmail(ctx, sub)
	if err != nil {
		slog.Error("cannot read a principal's email", "sub", sub, "err", err)
		return ""
	}
	p.mu.Lock()
	if p.known == nil {
		p.known = map[string]string{}
	}
	p.known[sub] = email // "" too, until learn says otherwise
	p.mu.Unlock()
	return email
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
// one for midgard from someone the allowlist admits; errNotOnList when it is
// good, but its principal is not admitted. The owner of everything they
// store is its principal id, Sub, which, unlike an email, never changes.
func (a *latereAuth) identify(r *http.Request) (id authkit.Identity, err error) {
	if a == nil {
		return id, errors.New("no sign-in through auth.latere.ai")
	}
	id, err = a.auth.Authenticate(r)
	if err != nil {
		return id, err
	}
	if id.Sub == "" {
		return id, errors.New("a token for no one")
	}
	if !a.permits(id.Sub, id.Email) {
		slog.Warn("latere principal not allowed", "sub", id.Sub, "email", id.Email)
		return id, errNotOnList
	}
	return id, nil
}

// permits reports whether the allowlist admits the principal sub, known by
// email, or, when the token says no email, by the one they signed in on the
// web page with. A nil receiver admits no one.
func (a *latereAuth) permits(sub, email string) bool {
	if a == nil {
		return false
	}
	if a.allowed[strings.ToLower(sub)] {
		return true
	}
	if email == "" {
		email = a.emails.emailOf(sub)
	}
	return email != "" && a.allowed[strings.ToLower(email)]
}

// learn records that sub signed in on the web page as email.
func (a *latereAuth) learn(sub, email string) {
	if a != nil {
		a.emails.learn(sub, email)
	}
}
