// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"
)

// A person's history is bounded three ways, and a new copy pushes out the
// oldest past any of them (specs/redesign.md §6).
var (
	// HistoryLength is how many copies are kept.
	HistoryLength = 200
	// HistoryAge is how long a copy is kept.
	HistoryAge = 30 * 24 * time.Hour
	// HistoryBytes is how much a person's history may hold, images and all.
	HistoryBytes int64 = 64 << 20
)

// Clip is one copy in a person's history.
type Clip struct {
	ID      int64
	Device  string // the device it was copied on
	Created time.Time
	MIME    string
	Data    []byte // left out of History's listing; see Clip
	Size    int
}

// AddClip adds a copy to owner's history, making it their clipboard, and
// reports whether it changed anything: a copy identical to the newest is not
// added again, which is what stops a copy echoing between devices. Copies a
// password manager marked never get here; the daemon drops them.
func (s *Store) AddClip(ctx context.Context, owner, device, mime string, data []byte) (bool, error) {
	if owner == "" {
		return false, errors.New("store: a copy needs an owner")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var lastMIME string
	var lastData []byte
	err = tx.QueryRowContext(ctx,
		`SELECT mime, data FROM clips WHERE owner = ? ORDER BY id DESC LIMIT 1`, owner).Scan(&lastMIME, &lastData)
	if err == nil && lastMIME == mime && bytes.Equal(lastData, data) {
		return false, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	now := time.Now()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO clips (owner, device, created, mime, data) VALUES (?, ?, ?, ?, ?)`,
		owner, device, now.UnixMilli(), mime, data); err != nil {
		return false, err
	}
	if err := prune(ctx, tx, owner, now); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// prune keeps owner's history inside its three bounds. The newest copy always
// stays, even alone past the byte budget: it is the clipboard.
func prune(ctx context.Context, tx *sql.Tx, owner string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM clips WHERE owner = ? AND id NOT IN (
			SELECT id FROM clips WHERE owner = ? ORDER BY id DESC LIMIT ?)`,
		owner, owner, HistoryLength); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM clips WHERE owner = ? AND created < ? AND id <> (
			SELECT MAX(id) FROM clips WHERE owner = ?)`,
		owner, now.Add(-HistoryAge).UnixMilli(), owner); err != nil {
		return err
	}
	// Past the byte budget, drop the oldest until it fits.
	_, err := tx.ExecContext(ctx, `
		DELETE FROM clips WHERE owner = ? AND id IN (
			SELECT id FROM (
				SELECT id, SUM(LENGTH(data)) OVER (ORDER BY id DESC) AS running
				FROM clips WHERE owner = ?)
			WHERE running > ? AND id <> (SELECT MAX(id) FROM clips WHERE owner = ?))`,
		owner, owner, HistoryBytes, owner)
	return err
}

// LatestClip is owner's clipboard: the newest copy in their history, or
// ErrNotFound when they have none.
func (s *Store) LatestClip(ctx context.Context, owner string) (Clip, error) {
	return s.clip(ctx, `SELECT id, device, created, mime, data FROM clips WHERE owner = ? ORDER BY id DESC LIMIT 1`, owner)
}

// Clip is the copy numbered id in owner's history, or ErrNotFound: one in
// someone else's is not theirs to see, and not told apart from none.
func (s *Store) Clip(ctx context.Context, owner string, id int64) (Clip, error) {
	return s.clip(ctx, `SELECT id, device, created, mime, data FROM clips WHERE owner = ? AND id = ?`, owner, id)
}

func (s *Store) clip(ctx context.Context, query string, args ...any) (Clip, error) {
	var c Clip
	var created int64
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&c.ID, &c.Device, &created, &c.MIME, &c.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return Clip{}, ErrNotFound
	}
	if err != nil {
		return Clip{}, err
	}
	c.Created, c.Size = time.UnixMilli(created).UTC(), len(c.Data)
	return c, nil
}

// History lists owner's copies, newest first, without their data.
func (s *Store) History(ctx context.Context, owner string) ([]Clip, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, device, created, mime, LENGTH(data) FROM clips WHERE owner = ? ORDER BY id DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Clip{}
	for rows.Next() {
		var c Clip
		var created int64
		if err := rows.Scan(&c.ID, &c.Device, &created, &c.MIME, &c.Size); err != nil {
			return nil, err
		}
		c.Created = time.UnixMilli(created).UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteClip removes the copy numbered id from owner's history.
func (s *Store) DeleteClip(ctx context.Context, owner string, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM clips WHERE owner = ? AND id = ?`, owner, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearHistory removes all of owner's history.
func (s *Store) ClearHistory(ctx context.Context, owner string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM clips WHERE owner = ?`, owner)
	return err
}
