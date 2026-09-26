// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package history is the clipboard history a device keeps: its copy of its
// person's history, which the server numbers and does not keep
// (specs/redesign.md §6, §7).
//
// The history is a log of numbered events: copies, and the deletes and
// clears that remove them. Every device applies the same events and trims
// the same way, so each shows the same list in the same order: by when a
// copy was made, then by its number. A copy removed stays in the log as a
// tombstone, without its bytes, so the device can tell a peer catching up
// that it is gone. What the device does before the server has numbered it
// waits in an outbox.
package history

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"changkun.de/x/midgard/internal/wire"
	_ "modernc.org/sqlite"
)

// The bounds of a history (§6). A copy past any of them is removed, the same
// on every device.
const (
	MaxCopies = 200
	MaxAge    = 30 * 24 * time.Hour
	MaxBytes  = 64 << 20
)

// ErrNotFound is returned for a copy the history does not have.
var ErrNotFound = errors.New("history: no such copy")

// Store is a device's history, in a SQLite file of its own.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// Entry is one copy in the history.
type Entry struct {
	Seq     uint64 // its number; 0 while it waits to be numbered
	Ref     string // the device's own name for it, while it waits
	Time    time.Time
	Origin  string
	Formats []wire.Format
	Data    []byte // the formats' bytes; left out of List
}

// Size is how many bytes the copy holds.
func (e Entry) Size() int {
	n := 0
	for _, f := range e.Formats {
		n += f.Size
	}
	return n
}

// Waiting reports whether the copy is still to be numbered by the server.
func (e Entry) Waiting() bool { return e.Seq == 0 }

// Frame is e as a copy to send, or an event to answer a peer with.
func (e Entry) Frame() wire.Frame {
	f := wire.Frame{
		Envelope: wire.Envelope{
			Type: wire.Event, Kind: wire.KindCopy, Seq: e.Seq, Ref: e.Ref,
			Time: e.Time.UnixMilli(), Origin: e.Origin, Formats: e.Formats,
		},
		Payload: e.Data,
	}
	if e.Waiting() {
		f.Type = wire.Copy
	}
	return f
}

// Open opens the history at path, creating it if need be. Only its user may
// read it: it holds what they copied.
func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, now: time.Now}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("history: %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the history.
func (s *Store) Close() error { return s.db.Close() }

var migrations = []string{
	// 1: the numbered log, and what waits to be numbered. A removed copy
	// keeps its row, gone and without data.
	`CREATE TABLE events (
		seq     INTEGER PRIMARY KEY,
		kind    TEXT    NOT NULL,
		time    INTEGER NOT NULL,
		origin  TEXT    NOT NULL DEFAULT '',
		target  INTEGER NOT NULL DEFAULT 0,
		formats TEXT    NOT NULL DEFAULT '[]',
		data    BLOB,
		gone    INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX events_live ON events (kind, gone, time, seq);
	CREATE TABLE outbox (
		n       INTEGER PRIMARY KEY AUTOINCREMENT,
		ref     TEXT    NOT NULL UNIQUE,
		type    TEXT    NOT NULL,
		time    INTEGER NOT NULL,
		offline INTEGER NOT NULL DEFAULT 0,
		target  INTEGER NOT NULL DEFAULT 0,
		formats TEXT    NOT NULL DEFAULT '[]',
		data    BLOB
	)`,
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("the history is at version %d, newer than this midgard knows (%d)", version, len(migrations))
	}
	for i := version; i < len(migrations); i++ {
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations))); err != nil {
		return err
	}
	return tx.Commit()
}

// Apply applies an event the server numbered, once: an event already in the
// log is left as it is. A copy that a delete or a clear already in the log
// covers arrives gone, as catching up can bring events out of order; a copy
// the device sent itself leaves the outbox. It reports whether the event was
// new, and whether it was the device's own, come back numbered.
func (s *Store) Apply(ctx context.Context, f wire.Frame) (applied, ours bool, err error) {
	if f.Seq == 0 {
		return false, false, errors.New("history: an event without a number")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, false, err
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM events WHERE seq = ?)`, f.Seq).Scan(&exists); err != nil {
		return false, false, err
	}
	if exists {
		return false, false, tx.Commit()
	}
	if f.Ref != "" {
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM outbox WHERE ref = ?)`, f.Ref).Scan(&ours); err != nil {
			return false, false, err
		}
	}
	formats, _ := marshalFormats(f.Formats)
	switch f.Kind {
	case wire.KindCopy:
		var covered bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM events
			WHERE (kind = 'delete' AND target = ?) OR (kind = 'clear' AND target >= ?))`, f.Seq, f.Seq).Scan(&covered)
		if err != nil {
			return false, false, err
		}
		var data any = f.Payload
		if covered {
			data = nil
		}
		if f.Payload == nil && !covered {
			data = []byte{}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO events (seq, kind, time, origin, formats, data, gone) VALUES (?, 'copy', ?, ?, ?, ?, ?)`,
			f.Seq, f.Time, f.Origin, formats, data, covered); err != nil {
			return false, false, err
		}
	case wire.KindDelete, wire.KindClear:
		if _, err := tx.ExecContext(ctx, `INSERT INTO events (seq, kind, time, origin, target) VALUES (?, ?, ?, ?, ?)`,
			f.Seq, f.Kind, f.Time, f.Origin, f.Target); err != nil {
			return false, false, err
		}
		where := `seq = ?`
		if f.Kind == wire.KindClear {
			where = `seq <= ?`
		}
		if _, err := tx.ExecContext(ctx, `UPDATE events SET gone = 1, data = NULL WHERE kind = 'copy' AND `+where, f.Target); err != nil {
			return false, false, err
		}
	case wire.KindVoid:
		if _, err := tx.ExecContext(ctx, `INSERT INTO events (seq, kind, time, origin, gone) VALUES (?, 'void', ?, ?, 1)`,
			f.Seq, f.Time, f.Origin); err != nil {
			return false, false, err
		}
	default:
		return false, false, fmt.Errorf("history: an event of kind %q", f.Kind)
	}
	if ours {
		if _, err := tx.ExecContext(ctx, `DELETE FROM outbox WHERE ref = ?`, f.Ref); err != nil {
			return false, false, err
		}
	}
	if err := s.trim(ctx, tx); err != nil {
		return false, false, err
	}
	return true, ours, tx.Commit()
}

// trim removes the copies past the bounds: those older than MaxAge, and past
// the newest MaxCopies or MaxBytes, in the history's order.
func (s *Store) trim(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT seq, time, LENGTH(data) FROM events
		WHERE kind = 'copy' AND gone = 0 ORDER BY time DESC, seq DESC`)
	if err != nil {
		return err
	}
	oldest := s.now().Add(-MaxAge).UnixMilli()
	var drop []uint64
	n, bytes := 0, 0
	for rows.Next() {
		var seq uint64
		var at int64
		var size sql.NullInt64
		if err := rows.Scan(&seq, &at, &size); err != nil {
			rows.Close()
			return err
		}
		n++
		bytes += int(size.Int64)
		if n > MaxCopies || bytes > MaxBytes || at < oldest {
			drop = append(drop, seq)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, seq := range drop {
		if _, err := tx.ExecContext(ctx, `UPDATE events SET gone = 1, data = NULL WHERE seq = ?`, seq); err != nil {
			return err
		}
	}
	return nil
}

// Acked is the highest number the device has applied: what it acknowledges
// to the server. What it lacks below it is in Gaps.
func (s *Store) Acked(ctx context.Context) (uint64, error) {
	var seq uint64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM events`).Scan(&seq)
	return seq, err
}

// Gaps are the events below Acked the device does not have, to catch up on.
// A gap whose next event is older than MaxAge is filled with voids instead:
// whatever it held is past the bounds and would be removed on arrival.
func (s *Store) Gaps(ctx context.Context) ([]wire.Span, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT seq, time FROM events ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	type ev struct {
		seq uint64
		at  int64
	}
	var all []ev
	for rows.Next() {
		var e ev
		if err := rows.Scan(&e.seq, &e.at); err != nil {
			rows.Close()
			return nil, err
		}
		all = append(all, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	oldest := s.now().Add(-MaxAge).UnixMilli()
	var gaps, expired []wire.Span
	next := uint64(1)
	for _, e := range all {
		if e.seq > next {
			span := wire.Span{From: next, To: e.seq - 1}
			if e.at < oldest {
				expired = append(expired, span)
			} else {
				gaps = append(gaps, span)
			}
		}
		next = e.seq + 1
	}
	if len(expired) > 0 {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()
		for _, span := range expired {
			for seq := span.From; seq <= span.To; seq++ {
				if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO events (seq, kind, time, gone) VALUES (?, 'void', 0, 1)`, seq); err != nil {
					return nil, err
				}
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}
	return gaps, nil
}

// Newest is the newest copy, the one that belongs on the clipboard, with its
// bytes. With waiting, a copy the server has not numbered yet counts too.
func (s *Store) Newest(ctx context.Context, waiting bool) (Entry, bool, error) {
	list, err := s.list(ctx, 1, waiting, true)
	if err != nil || len(list) == 0 {
		return Entry{}, false, err
	}
	return list[0], true, nil
}

// List is the newest n copies, newest first, without their bytes: those
// numbered, and those waiting to be.
func (s *Store) List(ctx context.Context, n int) ([]Entry, error) {
	return s.list(ctx, n, true, false)
}

func (s *Store) list(ctx context.Context, n int, waiting, data bool) ([]Entry, error) {
	col := `NULL`
	if data {
		col = `data`
	}
	query := `SELECT seq, '' AS ref, time, origin, formats, ` + col + ` AS data, 0 AS waiting
		FROM events WHERE kind = 'copy' AND gone = 0`
	if waiting {
		query += ` UNION ALL SELECT 0, ref, time, '', formats, ` + col + `, 1 FROM outbox WHERE type = 'copy'`
	}
	// A copy waiting to be numbered will be numbered after every copy of
	// the same time, so it sorts as the newer.
	query = `SELECT seq, ref, time, origin, formats, data FROM (` + query + `)
		ORDER BY time DESC, waiting DESC, seq DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Get is the copy numbered seq, with its bytes.
func (s *Store) Get(ctx context.Context, seq uint64) (Entry, error) {
	row := s.db.QueryRowContext(ctx, `SELECT seq, '', time, origin, formats, data FROM events
		WHERE seq = ? AND kind = 'copy' AND gone = 0`, seq)
	e, err := scanEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, ErrNotFound
	}
	return e, err
}

type scanner interface{ Scan(...any) error }

func scanEntry(r scanner) (Entry, error) {
	var e Entry
	var at int64
	var formats string
	var data []byte
	if err := r.Scan(&e.Seq, &e.Ref, &at, &e.Origin, &formats, &data); err != nil {
		return Entry{}, err
	}
	e.Time = time.UnixMilli(at)
	if err := json.Unmarshal([]byte(formats), &e.Formats); err != nil {
		return Entry{}, err
	}
	if data != nil {
		e.Data = data
	}
	return e, nil
}

// Events are the events in span the device holds, to answer a peer catching
// up: a removed copy as a void, so the peer knows it is gone.
func (s *Store) Events(ctx context.Context, span wire.Span) ([]wire.Frame, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT seq, kind, time, origin, target, formats, data, gone FROM events
		WHERE seq BETWEEN ? AND ? ORDER BY seq`, span.From, span.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []wire.Frame
	for rows.Next() {
		var f wire.Frame
		var formats string
		var gone bool
		if err := rows.Scan(&f.Seq, &f.Kind, &f.Time, &f.Origin, &f.Target, &formats, &f.Payload, &gone); err != nil {
			return nil, err
		}
		f.Type = wire.Event
		if gone && f.Kind == wire.KindCopy {
			f.Kind, f.Payload = wire.KindVoid, nil
		}
		if f.Kind == wire.KindCopy {
			if err := json.Unmarshal([]byte(formats), &f.Formats); err != nil {
				return nil, err
			}
		}
		if len(f.Payload) == 0 {
			f.Payload = nil
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Add records what happened on the device itself, to send to the server:
// a copy, or the delete or clear of copies. It takes effect here at once,
// and waits in the outbox until the server numbers it. offline says the
// device was not connected when it happened. It returns the frame to send.
func (s *Store) Add(ctx context.Context, f wire.Frame, offline bool) (wire.Frame, error) {
	ref, err := newRef()
	if err != nil {
		return wire.Frame{}, err
	}
	f.Ref, f.Offline = ref, offline
	f.Time = s.now().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wire.Frame{}, err
	}
	defer tx.Rollback()

	switch f.Type {
	case wire.Copy:
		f.Kind = wire.KindCopy
	case wire.Delete:
		f.Kind = wire.KindDelete
		if _, err := tx.ExecContext(ctx, `UPDATE events SET gone = 1, data = NULL WHERE seq = ? AND kind = 'copy'`, f.Target); err != nil {
			return wire.Frame{}, err
		}
	case wire.Clear:
		// The server clears what it has numbered by the time the clear
		// reaches it, which may be more than this device has.
		f.Kind = wire.KindClear
		if _, err := tx.ExecContext(ctx, `UPDATE events SET gone = 1, data = NULL WHERE kind = 'copy'`); err != nil {
			return wire.Frame{}, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM outbox WHERE type = 'copy'`); err != nil {
			return wire.Frame{}, err
		}
	default:
		return wire.Frame{}, fmt.Errorf("history: cannot add a %q", f.Type)
	}
	formats, _ := marshalFormats(f.Formats)
	data := f.Payload
	if data == nil {
		data = []byte{}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO outbox (ref, type, time, offline, target, formats, data) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.Ref, f.Type, f.Time, offline, f.Target, formats, data); err != nil {
		return wire.Frame{}, err
	}
	return f, tx.Commit()
}

// Forget takes back a copy still waiting in the outbox, before the server
// has it.
func (s *Store) Forget(ctx context.Context, ref string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM outbox WHERE ref = ?`, ref)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Outbox is what waits to be numbered, in the order it happened, to send
// again once connected.
func (s *Store) Outbox(ctx context.Context) ([]wire.Frame, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ref, type, time, offline, target, formats, data FROM outbox ORDER BY n`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []wire.Frame
	for rows.Next() {
		var f wire.Frame
		var formats string
		if err := rows.Scan(&f.Ref, &f.Type, &f.Time, &f.Offline, &f.Target, &formats, &f.Payload); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(formats), &f.Formats); err != nil {
			return nil, err
		}
		f.Kind = map[wire.Type]wire.Kind{wire.Copy: wire.KindCopy, wire.Delete: wire.KindDelete, wire.Clear: wire.KindClear}[f.Type]
		if len(f.Payload) == 0 {
			f.Payload = nil
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// marshalFormats encodes formats, none as an empty list.
func marshalFormats(formats []wire.Format) ([]byte, error) {
	if formats == nil {
		formats = []wire.Format{}
	}
	return json.Marshal(formats)
}

func newRef() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
