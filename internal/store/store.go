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
	// websocket and REST paths write concurrently.
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
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
