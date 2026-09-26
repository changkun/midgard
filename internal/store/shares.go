// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"math/big"
	"strings"
	"time"
)

// Share is a copy or a file published at a link (specs/redesign.md §9).
// Shares are public by design: anyone with the link reads it. Only its owner
// lists or revokes it.
type Share struct {
	Slug    string    // its random link: /midgard/s/<slug>
	Path    string    // its name, when it has one: /midgard/<path>
	Owner   string    // whose it is
	Created time.Time //
	Expires time.Time // zero: never
	MIME    string
	Data    []byte // left out of Shares' listing
	Size    int
}

// ErrTaken is returned when a share's name is already someone's.
var ErrTaken = errors.New("store: the name is taken")

// CreateShare publishes data for owner at a new random link, and at path too
// when it is not empty. Names are one namespace for everyone, first come
// first served, as they always were, until the share expires or is revoked.
// A zero expires never expires.
func (s *Store) CreateShare(ctx context.Context, owner, path, mime string, data []byte, expires time.Time) (Share, error) {
	if owner == "" {
		return Share{}, errors.New("store: a share needs an owner")
	}
	slug, err := newSlug()
	if err != nil {
		return Share{}, err
	}
	sh := Share{Slug: slug, Path: path, Owner: owner, Created: time.Now(), Expires: expires, MIME: mime, Data: data, Size: len(data)}
	if path != "" {
		// a name is free again once the share that had it has expired
		_, err := s.db.ExecContext(ctx, `DELETE FROM shares WHERE path = ? AND expires <= ?`, path, sh.Created.UnixMilli())
		if err != nil {
			return Share{}, err
		}
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO shares (slug, path, owner, created, expires, mime, data) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		slug, nullable(path), owner, sh.Created.UnixMilli(), unixOrNull(expires), mime, data)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: shares.path") {
		return Share{}, ErrTaken
	}
	return sh, err
}

// ShareBySlug and ShareByPath look a share up by its link, for anyone: it is
// public. One that has expired is not found.
func (s *Store) ShareBySlug(ctx context.Context, slug string) (Share, error) {
	return s.share(ctx, `slug = ?`, slug)
}

// ShareByPath looks a share up by its name; see ShareBySlug.
func (s *Store) ShareByPath(ctx context.Context, path string) (Share, error) {
	return s.share(ctx, `path = ?`, path)
}

func (s *Store) share(ctx context.Context, where string, key string) (Share, error) {
	var sh Share
	var path sql.NullString
	var created int64
	var expires sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT slug, path, owner, created, expires, mime, data FROM shares WHERE `+where, key).
		Scan(&sh.Slug, &path, &sh.Owner, &created, &expires, &sh.MIME, &sh.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return Share{}, ErrNotFound
	}
	if err != nil {
		return Share{}, err
	}
	sh.Path, sh.Created, sh.Size = path.String, time.UnixMilli(created).UTC(), len(sh.Data)
	if expires.Valid {
		sh.Expires = time.UnixMilli(expires.Int64).UTC()
		if time.Now().After(sh.Expires) {
			return Share{}, ErrNotFound
		}
	}
	return sh, nil
}

// Shares lists owner's shares, newest first, without their data, expired ones
// included so their owner can see them go.
func (s *Store) Shares(ctx context.Context, owner string) ([]Share, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT slug, path, created, expires, mime, LENGTH(data) FROM shares WHERE owner = ? ORDER BY created DESC, slug`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Share{}
	for rows.Next() {
		sh := Share{Owner: owner}
		var path sql.NullString
		var created int64
		var expires sql.NullInt64
		if err := rows.Scan(&sh.Slug, &path, &created, &expires, &sh.MIME, &sh.Size); err != nil {
			return nil, err
		}
		sh.Path, sh.Created = path.String, time.UnixMilli(created).UTC()
		if expires.Valid {
			sh.Expires = time.UnixMilli(expires.Int64).UTC()
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// DeleteShare revokes owner's share slug. Someone else's is not found.
func (s *Store) DeleteShare(ctx context.Context, owner, slug string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM shares WHERE owner = ? AND slug = ?`, owner, slug)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// newSlug is a random link of 22 characters from [0-9A-Za-z], about 131 bits.
func newSlug() (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 22)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		b[i] = alphabet[n.Int64()]
	}
	return string(b), nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func unixOrNull(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UnixMilli()
}
