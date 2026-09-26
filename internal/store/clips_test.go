// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestHistory(t *testing.T) {
	s, _ := open(t)
	ctx := context.Background()

	if _, err := s.LatestClip(ctx, "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an empty history: %v, want ErrNotFound", err)
	}
	for _, text := range []string{"one", "two", "three"} {
		if changed, err := s.AddClip(ctx, "alice", "laptop", "text/plain", []byte(text)); err != nil || !changed {
			t.Fatalf("AddClip(%q) = %v, %v", text, changed, err)
		}
	}
	// the same copy again changes nothing: that stops echoes between devices
	if changed, _ := s.AddClip(ctx, "alice", "phone", "text/plain", []byte("three")); changed {
		t.Error("a copy identical to the newest was added again")
	}

	latest, err := s.LatestClip(ctx, "alice")
	if err != nil || string(latest.Data) != "three" || latest.Device != "laptop" {
		t.Fatalf("LatestClip = %+v, %v; want three from the laptop", latest, err)
	}
	h, _ := s.History(ctx, "alice")
	if len(h) != 3 || h[0].ID != latest.ID || h[0].Data != nil || h[0].Size != 5 {
		t.Fatalf("History = %+v; want three entries, newest first, sized, without data", h)
	}
	if c, err := s.Clip(ctx, "alice", h[2].ID); err != nil || string(c.Data) != "one" {
		t.Fatalf("Clip(oldest) = %+v, %v", c, err)
	}

	if err := s.DeleteClip(ctx, "alice", latest.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.LatestClip(ctx, "alice"); string(l.Data) != "two" {
		t.Errorf("after deleting the newest, the clipboard is %q, want two", l.Data)
	}
	if err := s.ClearHistory(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if h, _ := s.History(ctx, "alice"); len(h) != 0 {
		t.Errorf("after clearing, %d entries are left", len(h))
	}
}

// TestHistoryKeepsOwnersApart is the barrier for history: no reading,
// deleting or clearing of someone else's.
func TestHistoryKeepsOwnersApart(t *testing.T) {
	s, _ := open(t)
	ctx := context.Background()
	s.AddClip(ctx, "alice", "laptop", "text/plain", []byte("alice's"))
	s.AddClip(ctx, "bob", "laptop", "text/plain", []byte("bob's"))
	alices, _ := s.LatestClip(ctx, "alice")

	if l, _ := s.LatestClip(ctx, "bob"); string(l.Data) != "bob's" {
		t.Fatalf("bob's clipboard is %q", l.Data)
	}
	if h, _ := s.History(ctx, "bob"); len(h) != 1 {
		t.Fatalf("bob's history has %d entries, want his one", len(h))
	}
	if _, err := s.Clip(ctx, "bob", alices.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob reading alice's copy by its id: %v, want ErrNotFound", err)
	}
	if err := s.DeleteClip(ctx, "bob", alices.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob deleting alice's copy: %v, want ErrNotFound", err)
	}
	s.ClearHistory(ctx, "bob")
	if l, err := s.LatestClip(ctx, "alice"); err != nil || string(l.Data) != "alice's" {
		t.Error("bob clearing his history touched alice's")
	}
	// the same copy by two people is two copies, not an echo
	if changed, _ := s.AddClip(ctx, "bob", "laptop", "text/plain", []byte("alice's")); !changed {
		t.Error("bob copying the text alice copied was taken for an echo")
	}
}

func TestHistoryBounds(t *testing.T) {
	saved := [...]any{HistoryLength, HistoryAge, HistoryBytes}
	t.Cleanup(func() {
		HistoryLength, HistoryAge, HistoryBytes = saved[0].(int), saved[1].(time.Duration), saved[2].(int64)
	})
	ctx := context.Background()

	t.Run("length", func(t *testing.T) {
		s, _ := open(t)
		HistoryLength = 3
		for i := range 5 {
			s.AddClip(ctx, "alice", "laptop", "text/plain", fmt.Appendf(nil, "copy %d", i))
		}
		h, _ := s.History(ctx, "alice")
		if len(h) != 3 {
			t.Fatalf("%d entries, want 3", len(h))
		}
		if l, _ := s.LatestClip(ctx, "alice"); string(l.Data) != "copy 4" {
			t.Fatalf("the newest is %q", l.Data)
		}
		HistoryLength = saved[0].(int)
	})
	t.Run("bytes", func(t *testing.T) {
		s, _ := open(t)
		HistoryBytes = 10
		s.AddClip(ctx, "alice", "laptop", "image/png", make([]byte, 6))
		s.AddClip(ctx, "alice", "laptop", "image/png", make([]byte, 7))
		if h, _ := s.History(ctx, "alice"); len(h) != 1 || h[0].Size != 7 {
			t.Fatalf("history = %+v; want only the newest, past the budget", h)
		}
		// the newest stays even alone past the budget: it is the clipboard
		s.AddClip(ctx, "alice", "laptop", "image/png", make([]byte, 50))
		if l, err := s.LatestClip(ctx, "alice"); err != nil || l.Size != 50 {
			t.Fatalf("the clipboard went with the budget: %+v, %v", l, err)
		}
		HistoryBytes = saved[2].(int64)
	})
	t.Run("age", func(t *testing.T) {
		s, _ := open(t)
		s.AddClip(ctx, "alice", "laptop", "text/plain", []byte("old"))
		s.db.Exec(`UPDATE clips SET created = ?`, time.Now().Add(-48*time.Hour).UnixMilli())
		HistoryAge = 24 * time.Hour
		s.AddClip(ctx, "alice", "laptop", "text/plain", []byte("new"))
		if h, _ := s.History(ctx, "alice"); len(h) != 1 {
			t.Fatalf("%d entries; the old copy outlived HistoryAge", len(h))
		}
		HistoryAge = saved[1].(time.Duration)
	})
}
