// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TokenPrefix starts every app token, so that one is recognizable in a
// config file or a leaked log.
const TokenPrefix = "mgt_"

// AppToken describes an issued app token, without the token itself.
//
// App tokens are for the clients that cannot sign in on their own, such as an
// iOS Shortcut: named, shown once, revocable one at a time, and acting for
// their owner only. Only their SHA-256 is kept.
type AppToken struct {
	Name    string
	Created time.Time
}

// ErrNotFound is returned when there is nothing under the name asked for.
var ErrNotFound = errors.New("store: not found")

// TokenHolder is whom an app token acts for.
type TokenHolder struct {
	Owner string // the principal id
	Email string // the owner's email, when it was known at issue
	Name  string // the token's name
}

// IssueAppToken issues a token named name for owner and returns it. It is
// shown only this once; the store keeps its hash. email, which may be empty,
// is the owner's, for an allowlist that names people by email.
func (s *Store) IssueAppToken(ctx context.Context, owner, email, name string) (string, error) {
	if owner == "" {
		return "", errors.New("store: an app token needs an owner")
	}
	if err := validName(name); err != nil {
		return "", err
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	tok := TokenPrefix + base64.RawURLEncoding.EncodeToString(raw[:])
	h := sha256.Sum256([]byte(tok))
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO app_tokens (owner, email, name, hash, created) VALUES (?, ?, ?, ?, ?)`,
		owner, email, name, h[:], time.Now().Unix())
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: app_tokens.owner, app_tokens.name") {
		return "", fmt.Errorf("%q already has a token; revoke it first", name)
	}
	return tok, err
}

// AppTokens lists owner's app tokens, oldest first.
func (s *Store) AppTokens(ctx context.Context, owner string) ([]AppToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, created FROM app_tokens WHERE owner = ? ORDER BY created, name`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppToken
	for rows.Next() {
		var t AppToken
		var created int64
		if err := rows.Scan(&t.Name, &created); err != nil {
			return nil, err
		}
		t.Created = time.Unix(created, 0).UTC()
		out = append(out, t)
	}
	return out, rows.Err()
}

// RevokeAppToken revokes owner's token named name. Another owner's token of
// the same name is not touched, and not revealed: it is ErrNotFound either way.
func (s *Store) RevokeAppToken(ctx context.Context, owner, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM app_tokens WHERE owner = ? AND name = ?`, owner, name)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CheckAppToken finds whom tok acts for. It is the one lookup that takes no
// owner, because finding the owner is what it is for. The hash is looked up,
// not compared in Go, so how long a check takes says nothing about the token.
func (s *Store) CheckAppToken(ctx context.Context, tok string) (TokenHolder, bool) {
	var h TokenHolder
	if !strings.HasPrefix(tok, TokenPrefix) {
		return h, false
	}
	sum := sha256.Sum256([]byte(tok))
	err := s.db.QueryRowContext(ctx,
		`SELECT owner, email, name FROM app_tokens WHERE hash = ?`, sum[:]).Scan(&h.Owner, &h.Email, &h.Name)
	if err != nil { // sql.ErrNoRows among them
		return TokenHolder{}, false
	}
	return h, true
}

// validName accepts names that read well in a listing: letters, digits, and
// - _ . only.
func validName(name string) error {
	if name == "" || len(name) > 64 {
		return errors.New("a token name must be 1 to 64 characters")
	}
	for _, r := range name {
		ok := r == '-' || r == '_' || r == '.' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return fmt.Errorf("invalid token name %q: use letters, digits, - _ and . only", name)
		}
	}
	return nil
}
