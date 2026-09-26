// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/wire"
)

func TestNextSeq(t *testing.T) {
	s, path := open(t)
	ctx := context.Background()
	if h, _ := s.Head(ctx, "alice"); h != 0 {
		t.Fatalf("Head before any = %d", h)
	}
	// numbers are per person, and never given out twice, however many ask
	var wg sync.WaitGroup
	seen := make(chan uint64, 50)
	for range 50 {
		wg.Go(func() {
			n, err := s.NextSeq(ctx, "alice")
			if err != nil {
				t.Error(err)
			}
			seen <- n
		})
	}
	wg.Wait()
	close(seen)
	got := map[uint64]bool{}
	for n := range seen {
		if got[n] {
			t.Fatalf("%d given out twice", n)
		}
		got[n] = true
	}
	if len(got) != 50 || !got[1] || !got[50] {
		t.Fatalf("numbers %v, want 1 to 50", got)
	}
	if n, _ := s.NextSeq(ctx, "bob"); n != 1 {
		t.Fatalf("bob's first number is %d", n)
	}

	// and they survive a restart
	s.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, _ := s.NextSeq(ctx, "alice"); n != 51 {
		t.Fatalf("after reopening, alice's next number is %d, want 51", n)
	}
}

func TestDevices(t *testing.T) {
	s, _ := open(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	s.SeeDevice(ctx, "alice", Device{ID: "a1", Name: "laptop", LastSeen: at, Acked: 7, Gaps: []wire.Span{{From: 2, To: 3}}})
	s.SeeDevice(ctx, "alice", Device{ID: "a2", Name: "desktop", LastSeen: at.Add(time.Minute)})
	s.SeeDevice(ctx, "bob", Device{ID: "b1", Name: "laptop", LastSeen: at})

	ds, err := s.Devices(ctx, "alice")
	if err != nil || len(ds) != 2 || ds[0].ID != "a2" || ds[1].Acked != 7 || len(ds[1].Gaps) != 1 || !ds[1].LastSeen.Equal(at) {
		t.Fatalf("Devices = %+v, %v", ds, err)
	}
	if err := s.ForgetDevice(ctx, "alice", "b1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("alice forgetting bob's device: %v", err)
	}
	if err := s.ForgetDevice(ctx, "alice", "a1"); err != nil {
		t.Fatal(err)
	}
	if ds, _ := s.Devices(ctx, "alice"); !ds[1].Forgotten {
		t.Fatalf("a1 not forgotten: %+v", ds[1])
	}
	// connecting again, it counts again
	s.SeeDevice(ctx, "alice", Device{ID: "a1", Name: "laptop", LastSeen: at.Add(time.Hour), Acked: 9})
	if ds, _ := s.Devices(ctx, "alice"); ds[0].ID != "a1" || ds[0].Forgotten || ds[0].Acked != 9 || len(ds[0].Gaps) != 0 {
		t.Fatalf("a1 after connecting again: %+v", ds[0])
	}
}
