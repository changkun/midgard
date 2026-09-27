// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"changkun.de/x/midgard/internal/wire"
)

// What the relay keeps between restarts (specs/redesign.md §6, §7): for each
// person, the last number it gave out, and their devices. None of it is a
// copy.

// NextSeq gives out owner's next number. It is on disk when NextSeq returns,
// before anything is sent with it, so a number is never given out twice.
func (s *Store) NextSeq(ctx context.Context, owner string) (uint64, error) {
	var seq uint64
	err := s.db.QueryRowContext(ctx, `INSERT INTO heads (owner, seq) VALUES (?, 1)
		ON CONFLICT (owner) DO UPDATE SET seq = seq + 1 RETURNING seq`, owner).Scan(&seq)
	return seq, err
}

// Head is the last number owner was given, 0 if none.
func (s *Store) Head(ctx context.Context, owner string) (uint64, error) {
	var seq uint64
	err := s.db.QueryRowContext(ctx, `SELECT seq FROM heads WHERE owner = ?`, owner).Scan(&seq)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return seq, err
}

// Device is one of a person's devices: one install.
type Device struct {
	ID        string
	Name      string
	LastSeen  time.Time
	Acked     uint64      // the highest number it has applied
	Gaps      []wire.Span // what it lacks below Acked
	Forgotten bool        // its person forgot it; it no longer counts
}

// SeeDevice records that owner's device d is connected and has what d says,
// and remembers it again if it was forgotten.
func (s *Store) SeeDevice(ctx context.Context, owner string, d Device) error {
	gaps, err := json.Marshal(d.Gaps)
	if err != nil {
		return err
	}
	if d.Gaps == nil {
		gaps = []byte("[]")
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO devices (owner, id, name, last_seen, acked, gaps) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (owner, id) DO UPDATE SET name = excluded.name, last_seen = excluded.last_seen,
			acked = excluded.acked, gaps = excluded.gaps, forgotten = 0`,
		owner, d.ID, d.Name, d.LastSeen.UnixMilli(), d.Acked, gaps)
	return err
}

// Devices are owner's devices, the most recently seen first.
func (s *Store) Devices(ctx context.Context, owner string) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, last_seen, acked, gaps, forgotten FROM devices
		WHERE owner = ? ORDER BY last_seen DESC, id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		var seen int64
		var gaps string
		if err := rows.Scan(&d.ID, &d.Name, &seen, &d.Acked, &gaps, &d.Forgotten); err != nil {
			return nil, err
		}
		d.LastSeen = time.UnixMilli(seen).UTC()
		if err := json.Unmarshal([]byte(gaps), &d.Gaps); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ForgetDevice forgets owner's device id: the relay stops holding copies for
// it. It counts again if it connects again.
func (s *Store) ForgetDevice(ctx context.Context, owner, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE devices SET forgotten = 1 WHERE owner = ? AND id = ?`, owner, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Kid is the id of owner's key, "" while they have none (§11).
func (s *Store) Kid(ctx context.Context, owner string) (string, error) {
	var kid string
	err := s.db.QueryRowContext(ctx, `SELECT kid FROM keys WHERE owner = ?`, owner).Scan(&kid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return kid, err
}

// SetKid sets owner's key id, unless one is set: two devices making a key at
// once, the first wins. It returns the key id owner has now.
func (s *Store) SetKid(ctx context.Context, owner, kid string) (string, error) {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO keys (owner, kid, created) VALUES (?, ?, ?) ON CONFLICT (owner) DO NOTHING`,
		owner, kid, time.Now().UnixMilli()); err != nil {
		return "", err
	}
	return s.Kid(ctx, owner)
}
