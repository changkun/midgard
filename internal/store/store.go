// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package store keeps what the midgard server knows in one SQLite file, and
// keeps each person's part of it apart from everyone else's.
//
// That barrier is in the method signatures: everything a person owns is
// stored under their owner key, the principal id their sign-in names, and
// every method that reads or changes it takes that key, which callers take
// from the authenticated request and never from what the request says. The
// only lookups without an owner are the ones that find one, such as checking
// an app token.
//
// It is pure Go (modernc.org/sqlite), so the server stays a static binary.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store is the server's database.
type Store struct {
	db *sql.DB
}

// Open opens the database at path, creating it if need be, and brings its
// schema up to date. Several processes may have it open at once, as the
// server and mg server token do.
func Open(path string) (*Store, error) {
	// The directory, not only the file, is kept to its owner: SQLite
	// creates the -wal and -shm files beside the database with whatever
	// the umask allows, and they hold what the database does.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil { // in case it existed, looser
		return nil, err
	}
	// WAL lets readers go on while one connection writes, and the busy
	// timeout makes a writer wait for another instead of failing: the
	// websocket and REST paths write concurrently. synchronous(FULL) makes
	// every commit durable before it returns, power loss included: a
	// number the relay gives out must never be given out twice (§6), and
	// writes are few.
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=synchronous(FULL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// migrations bring the schema from each version to the next; a database
// records how many it has had. Append, never edit: a database already past a
// migration will not run it again.
var migrations = []string{
	// 1: app tokens, for the clients that cannot sign in themselves.
	`CREATE TABLE app_tokens (
		owner   TEXT    NOT NULL,
		name    TEXT    NOT NULL,
		hash    BLOB    NOT NULL UNIQUE,
		created INTEGER NOT NULL,
		PRIMARY KEY (owner, name)
	)`,
	// 2: the owner's email, since an allowlist may name people by it.
	`ALTER TABLE app_tokens ADD COLUMN email TEXT NOT NULL DEFAULT ''`,
	// 3: each person's clipboard history; the newest is their clipboard.
	`CREATE TABLE clips (
		id      INTEGER PRIMARY KEY AUTOINCREMENT,
		owner   TEXT    NOT NULL,
		device  TEXT    NOT NULL,
		created INTEGER NOT NULL,
		mime    TEXT    NOT NULL,
		data    BLOB    NOT NULL
	);
	CREATE INDEX clips_by_owner ON clips (owner, id)`,
	// 4: shares, public by design: a random link each, and a name when
	// asked for one; only their owner may list or revoke them.
	`CREATE TABLE shares (
		slug    TEXT    PRIMARY KEY,
		path    TEXT    UNIQUE,
		owner   TEXT    NOT NULL,
		created INTEGER NOT NULL,
		expires INTEGER,
		mime    TEXT    NOT NULL,
		data    BLOB    NOT NULL
	);
	CREATE INDEX shares_by_owner ON shares (owner, created)`,
	// 5: the server keeps no clipboard data (specs/redesign.md §7): the
	// history moves to the devices, and what the server keeps instead is
	// the last number it gave out for each person, and their devices with
	// what each has.
	`DROP TABLE clips;
	CREATE TABLE heads (
		owner TEXT    PRIMARY KEY,
		seq   INTEGER NOT NULL
	);
	CREATE TABLE devices (
		owner     TEXT    NOT NULL,
		id        TEXT    NOT NULL,
		name      TEXT    NOT NULL,
		last_seen INTEGER NOT NULL,
		acked     INTEGER NOT NULL DEFAULT 0,
		gaps      TEXT    NOT NULL DEFAULT '[]',
		forgotten INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (owner, id)
	)`,
	// 6: the id of each person's key (specs/redesign.md §11), never the
	// key: set once, by their first device to seal, so a device with
	// another is told to pair.
	`CREATE TABLE keys (
		owner   TEXT    PRIMARY KEY,
		kid     TEXT    NOT NULL,
		created INTEGER NOT NULL
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
		return fmt.Errorf("the schema is at version %d, newer than this server knows (%d)", version, len(migrations))
	}
	for i := version; i < len(migrations); i++ {
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}
	// PRAGMA takes no placeholders; the value is an integer we made.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations))); err != nil {
		return err
	}
	return tx.Commit()
}
