// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestShares(t *testing.T) {
	s, _ := open(t)
	ctx := context.Background()

	random, err := s.CreateShare(ctx, "alice", "", "text", []byte("hi"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(random.Slug) != 22 || random.Path != "" {
		t.Fatalf("a share without a name: %+v", random)
	}
	named, err := s.CreateShare(ctx, "alice", "notes/a.txt", "text/plain", []byte("named"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if named.Slug == random.Slug {
		t.Fatal("two shares have the same link")
	}

	// both links are public: anyone may look them up
	if sh, err := s.ShareBySlug(ctx, random.Slug); err != nil || string(sh.Data) != "hi" || sh.Owner != "alice" {
		t.Fatalf("ShareBySlug = %+v, %v", sh, err)
	}
	if sh, err := s.ShareByPath(ctx, "notes/a.txt"); err != nil || string(sh.Data) != "named" {
		t.Fatalf("ShareByPath = %+v, %v", sh, err)
	}
	if sh, err := s.ShareBySlug(ctx, named.Slug); err != nil || string(sh.Data) != "named" {
		t.Fatalf("a named share lost its random link: %+v, %v", sh, err)
	}
	if _, err := s.ShareBySlug(ctx, "nosuchslug"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown link: %v, want ErrNotFound", err)
	}

	// names are first come, first served, across everyone
	if _, err := s.CreateShare(ctx, "bob", "notes/a.txt", "text", []byte("bob's"), time.Time{}); !errors.Is(err, ErrTaken) {
		t.Fatalf("taking alice's name: %v, want ErrTaken", err)
	}

	list, err := s.Shares(ctx, "alice")
	if err != nil || len(list) != 2 || list[0].Slug != named.Slug || list[0].Data != nil || list[0].Size != 5 {
		t.Fatalf("Shares = %+v, %v; want both, newest first, sized, without data", list, err)
	}
}

// TestSharesKeepOwnersApart is the barrier for shares: a link is public, but
// no one lists or revokes anyone else's.
func TestSharesKeepOwnersApart(t *testing.T) {
	s, _ := open(t)
	ctx := context.Background()

	sh, err := s.CreateShare(ctx, "alice", "a", "text", []byte("alice's"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Shares(ctx, "bob"); len(list) != 0 {
		t.Fatalf("bob lists %+v", list)
	}
	if err := s.DeleteShare(ctx, "bob", sh.Slug); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob revoking alice's share: %v, want ErrNotFound", err)
	}
	if _, err := s.ShareBySlug(ctx, sh.Slug); err != nil {
		t.Fatalf("bob's attempt revoked it: %v", err)
	}
	if err := s.DeleteShare(ctx, "alice", sh.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShareByPath(ctx, "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a revoked share is still served: %v", err)
	}
	if err := s.DeleteShare(ctx, "alice", sh.Slug); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoking it twice: %v, want ErrNotFound", err)
	}
	// a revoked share's name is free
	if _, err := s.CreateShare(ctx, "bob", "a", "text", []byte("bob's"), time.Time{}); err != nil {
		t.Fatalf("taking a revoked name: %v", err)
	}
}

func TestSharesExpire(t *testing.T) {
	s, _ := open(t)
	ctx := context.Background()

	past := time.Now().Add(-time.Minute)
	gone, err := s.CreateShare(ctx, "alice", "old", "text", []byte("old"), past)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShareBySlug(ctx, gone.Slug); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an expired share by its link: %v, want ErrNotFound", err)
	}
	if _, err := s.ShareByPath(ctx, "old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an expired share by its name: %v, want ErrNotFound", err)
	}
	// its owner still sees it, with when it went
	if list, _ := s.Shares(ctx, "alice"); len(list) != 1 || !list[0].Expires.Equal(past.Truncate(time.Millisecond)) {
		t.Fatalf("Shares = %+v; want the expired share with its expiry", list)
	}

	later := time.Now().Add(time.Hour)
	if _, err := s.CreateShare(ctx, "alice", "soon", "text", []byte("soon"), later); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShareByPath(ctx, "soon"); err != nil {
		t.Fatalf("a share before it expires: %v", err)
	}
	// an expired share's name is free again, to anyone
	if _, err := s.CreateShare(ctx, "bob", "old", "text", []byte("new"), time.Time{}); err != nil {
		t.Fatalf("taking an expired name: %v", err)
	}
	if sh, err := s.ShareByPath(ctx, "old"); err != nil || string(sh.Data) != "new" {
		t.Fatalf("ShareByPath(old) = %+v, %v; want bob's", sh, err)
	}
	if _, err := s.CreateShare(ctx, "bob", "soon", "text", []byte("x"), time.Time{}); !errors.Is(err, ErrTaken) {
		t.Fatalf("taking a name before its share expires: %v, want ErrTaken", err)
	}
}
