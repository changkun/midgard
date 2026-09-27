// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package history

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/wire"
)

var ctx = context.Background()

// t0 is when the tests' clock stands.
var t0 = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func open(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "midgard", "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return t0 }
	t.Cleanup(func() { s.Close() })
	return s
}

// copyAt is the event of a text copy numbered seq, made at.
func copyAt(seq uint64, at time.Time, text string) wire.Frame {
	f := wire.NewCopy("text", []byte(text))
	f.Type, f.Seq, f.Time, f.Origin = wire.Event, seq, at.UnixMilli(), "laptop"
	return f
}

func event(seq uint64, kind wire.Kind, target uint64) wire.Frame {
	return wire.Frame{Envelope: wire.Envelope{Type: wire.Event, Seq: seq, Kind: kind, Target: target, Time: t0.UnixMilli()}}
}

func apply(t *testing.T, s *Store, fs ...wire.Frame) {
	t.Helper()
	for _, f := range fs {
		if _, _, err := s.Apply(ctx, f); err != nil {
			t.Fatalf("Apply(%d): %v", f.Seq, err)
		}
	}
}

// texts is the history as its texts, newest first.
func texts(t *testing.T, s *Store) string {
	t.Helper()
	list, err := s.List(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range list {
		if e.Data != nil {
			t.Errorf("List gave the bytes of %d", e.Seq)
		}
		got, err := s.Get(ctx, e.Seq)
		switch {
		case e.Waiting():
			out = append(out, "("+e.Ref+")")
		case err != nil:
			t.Fatalf("Get(%d): %v", e.Seq, err)
		default:
			out = append(out, string(got.Data))
		}
	}
	return strings.Join(out, " ")
}

func TestOrder(t *testing.T) {
	s := open(t)
	apply(t, s,
		copyAt(1, t0.Add(-3*time.Minute), "one"),
		copyAt(2, t0.Add(-2*time.Minute), "two"),
		copyAt(3, t0.Add(-time.Minute), "three"),
	)
	if got := texts(t, s); got != "three two one" {
		t.Fatalf("history %q", got)
	}

	// A copy made offline at -150s reaches the server last, as 4. It takes
	// its place at when it was made, and does not take over the clipboard.
	apply(t, s, copyAt(4, t0.Add(-150*time.Second), "offline"))
	if got := texts(t, s); got != "three two offline one" {
		t.Fatalf("history %q, want the offline copy at when it was made", got)
	}
	if e, ok, _ := s.Newest(ctx, false); !ok || string(e.Data) != "three" || e.Seq != 3 {
		t.Fatalf("Newest = %+v, want three", e)
	}

	// the same time: the later number is the newer
	apply(t, s, copyAt(5, t0.Add(-time.Minute), "three, again"))
	if e, _, _ := s.Newest(ctx, false); string(e.Data) != "three, again" {
		t.Fatalf("Newest = %q, want the later of two at the same time", e.Data)
	}

	// once is enough: an event applied again changes nothing
	apply(t, s, copyAt(5, t0, "not this"))
	if e, _ := s.Get(ctx, 5); string(e.Data) != "three, again" {
		t.Fatalf("applying 5 again changed it to %q", e.Data)
	}
	if n, _ := s.Acked(ctx); n != 5 {
		t.Fatalf("Acked = %d, want 5", n)
	}
}

func TestRemove(t *testing.T) {
	s := open(t)
	apply(t, s,
		copyAt(1, t0.Add(-3*time.Minute), "one"),
		copyAt(2, t0.Add(-2*time.Minute), "two"),
		copyAt(3, t0.Add(-time.Minute), "three"),
		event(4, wire.KindDelete, 2),
	)
	if got := texts(t, s); got != "three one" {
		t.Fatalf("after deleting 2: %q", got)
	}
	if _, err := s.Get(ctx, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(2) after its delete: %v", err)
	}
	apply(t, s, event(5, wire.KindClear, 3), copyAt(6, t0, "after"))
	if got := texts(t, s); got != "after" {
		t.Fatalf("after clearing up to 3: %q", got)
	}

	// Catching up brings events out of order: a copy whose delete or clear
	// came first arrives gone.
	s = open(t)
	apply(t, s, event(3, wire.KindDelete, 1), event(4, wire.KindClear, 2))
	apply(t, s, copyAt(1, t0, "deleted before it came"), copyAt(2, t0, "cleared before it came"))
	if got := texts(t, s); got != "" {
		t.Fatalf("history %q, want the late copies gone", got)
	}

	// a void holds nothing, and fills its number
	apply(t, s, event(5, wire.KindVoid, 0))
	if gaps, _ := s.Gaps(ctx); len(gaps) != 0 {
		t.Fatalf("gaps %v", gaps)
	}

	// A peer catching up is told what is gone, not sent it.
	evs, err := s.Events(ctx, wire.Span{From: 1, To: 9})
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, e := range evs {
		if e.Payload != nil {
			t.Errorf("event %d carries %q", e.Seq, e.Payload)
		}
		kinds = append(kinds, string(e.Kind))
	}
	if got := strings.Join(kinds, " "); got != "void void delete clear void" {
		t.Fatalf("events %q", got)
	}
}

func TestEventsForAPeer(t *testing.T) {
	s := open(t)
	apply(t, s, copyAt(1, t0, "one"), copyAt(2, t0, "two"), copyAt(4, t0, "four"))
	evs, err := s.Events(ctx, wire.Span{From: 2, To: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 || evs[0].Seq != 2 || string(evs[0].Payload) != "two" || evs[1].Seq != 4 {
		t.Fatalf("events %+v, want 2 and 4, as it lacks 3", evs)
	}
	// what a peer gets is what it can apply
	p := open(t)
	apply(t, p, evs...)
	if got := texts(t, p); got != "four two" {
		t.Fatalf("the peer's history %q", got)
	}
}

func TestGaps(t *testing.T) {
	s := open(t)
	apply(t, s, copyAt(1, t0, "one"), copyAt(4, t0, "four"), copyAt(7, t0, "seven"))
	gaps, err := s.Gaps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 2 || gaps[0] != (wire.Span{From: 2, To: 3}) || gaps[1] != (wire.Span{From: 5, To: 6}) {
		t.Fatalf("gaps %v", gaps)
	}
	// Acked is the highest applied, gaps or not: one gap nobody can fill
	// must not hold everything after it back (§6).
	if n, _ := s.Acked(ctx); n != 7 {
		t.Fatalf("Acked = %d, want 7", n)
	}

	// A gap older than the bounds is not worth catching up on.
	s = open(t)
	old := t0.Add(-MaxAge - time.Hour)
	apply(t, s, copyAt(1, old, "old"), copyAt(5, old, "old too"), copyAt(9, t0, "new"))
	gaps, _ = s.Gaps(ctx)
	if len(gaps) != 1 || gaps[0] != (wire.Span{From: 6, To: 8}) {
		t.Fatalf("gaps %v, want only the one before a recent copy", gaps)
	}
	if gaps, _ = s.Gaps(ctx); len(gaps) != 1 {
		t.Fatalf("asked again: %v", gaps)
	}
}

func TestOutbox(t *testing.T) {
	s := open(t)
	apply(t, s, copyAt(1, t0.Add(-time.Minute), "from the phone"))

	sent, err := s.Add(ctx, wire.NewCopy("text", []byte("mine")), true)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Ref == "" || !sent.Offline || sent.Time != t0.UnixMilli() || sent.Type != wire.Copy {
		t.Fatalf("Add = %+v", sent.Envelope)
	}
	// it shows at once, and belongs on the clipboard, while it waits
	if got := texts(t, s); got != "("+sent.Ref+") from the phone" {
		t.Fatalf("history %q", got)
	}
	if e, _, _ := s.Newest(ctx, true); !e.Waiting() || string(e.Data) != "mine" {
		t.Fatalf("Newest(waiting) = %+v", e)
	}
	if e, _, _ := s.Newest(ctx, false); string(e.Data) != "from the phone" {
		t.Fatalf("Newest(numbered) = %q", e.Data)
	}
	out, _ := s.Outbox(ctx)
	if len(out) != 1 || out[0].Ref != sent.Ref || string(out[0].Payload) != "mine" || !out[0].Offline {
		t.Fatalf("Outbox = %+v", out)
	}

	// The server numbers it and sends it back: it leaves the outbox, and is
	// known for the device's own.
	back := copyAt(2, t0, "mine")
	back.Ref = sent.Ref
	if applied, ours, err := s.Apply(ctx, back); err != nil || !applied || !ours {
		t.Fatalf("Apply(its own) = %v, %v, %v", applied, ours, err)
	}
	if applied, _, _ := s.Apply(ctx, back); applied {
		t.Fatal("applied it twice")
	}
	// a ref from another device's outbox is not this one's
	other := copyAt(3, t0.Add(-time.Hour), "theirs")
	other.Ref = "someone-elses"
	if _, ours, _ := s.Apply(ctx, other); ours {
		t.Fatal("another device's copy counted as this one's")
	}
	if out, _ := s.Outbox(ctx); len(out) != 0 {
		t.Fatalf("the outbox still has %+v", out)
	}
	if got := texts(t, s); got != "mine from the phone theirs" {
		t.Fatalf("history %q", got)
	}

	// a delete takes effect here at once, and waits to be sent
	del, err := s.Add(ctx, wire.Frame{Envelope: wire.Envelope{Type: wire.Delete, Target: 1}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(t, s); got != "mine theirs" {
		t.Fatalf("after deleting 1 here: %q", got)
	}
	if out, _ := s.Outbox(ctx); len(out) != 1 || out[0].Type != wire.Delete || out[0].Target != 1 || out[0].Ref != del.Ref {
		t.Fatalf("Outbox = %+v", out)
	}

	// a copy that waits can be taken back
	w, _ := s.Add(ctx, wire.NewCopy("text", []byte("oops")), false)
	if err := s.Forget(ctx, w.Ref); err != nil {
		t.Fatal(err)
	}
	if err := s.Forget(ctx, w.Ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("forgetting it twice: %v", err)
	}

	// a clear removes everything, the waiting copies too
	s.Add(ctx, wire.NewCopy("text", []byte("waiting")), false)
	if _, err := s.Add(ctx, wire.Frame{Envelope: wire.Envelope{Type: wire.Clear}}, false); err != nil {
		t.Fatal(err)
	}
	if got := texts(t, s); got != "" {
		t.Fatalf("after a clear here: %q", got)
	}
	out, _ = s.Outbox(ctx)
	if len(out) != 2 || out[0].Type != wire.Delete || out[1].Type != wire.Clear {
		t.Fatalf("Outbox = %+v, want the delete and the clear", out)
	}
}

// TestOnce: the history holds a copy once, where it was copied last, the same
// on every device whatever order the events reach it in.
func TestOnce(t *testing.T) {
	s := open(t)
	apply(t, s,
		copyAt(1, t0.Add(-3*time.Minute), "a"),
		copyAt(2, t0.Add(-2*time.Minute), "b"),
		copyAt(3, t0.Add(-time.Minute), "a"), // copied again, or put back from the history
	)
	if got := texts(t, s); got != "a b" {
		t.Fatalf("history %q, want a once, as the newest", got)
	}
	if e, _, _ := s.Newest(ctx, false); e.Seq != 3 {
		t.Errorf("Newest = %d, want 3", e.Seq)
	}
	if _, err := s.Get(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(1) = %v, want it gone", err)
	}
	// and a peer catching up is told it is gone
	if fs, _ := s.Events(ctx, wire.Span{From: 1, To: 1}); len(fs) != 1 || fs[0].Kind != wire.KindVoid {
		t.Errorf("Events(1) = %+v, want a void", fs)
	}

	// made offline before the b that is there: it arrives gone
	apply(t, s, copyAt(4, t0.Add(-150*time.Second), "b"))
	if got := texts(t, s); got != "a b" {
		t.Fatalf("history %q, want the newer b kept", got)
	}
	if e, err := s.Get(ctx, 2); err != nil || string(e.Data) != "b" {
		t.Fatalf("Get(2) = %v, want the newer b", err)
	}

	// the same bytes as another type are another copy
	img := wire.NewCopy("image/png", []byte("a"))
	img.Type, img.Seq, img.Time = wire.Event, 5, t0.UnixMilli()
	apply(t, s, img)
	if list, _ := s.List(ctx, 10); len(list) != 3 {
		t.Fatalf("%d copies, want the image beside the text", len(list))
	}

	// out of order, as catching up brings them: the older arrives gone
	o := open(t)
	apply(t, o, copyAt(3, t0.Add(-time.Minute), "a"), copyAt(2, t0.Add(-2*time.Minute), "b"), copyAt(1, t0.Add(-3*time.Minute), "a"))
	if got := texts(t, o); got != "a b" {
		t.Fatalf("out of order: history %q, want a b", got)
	}
	// a copy that arrives deleted still removes the older ones, as it did
	// on the devices that had it before its delete
	apply(t, o, event(5, wire.KindDelete, 6), copyAt(6, t0, "b"))
	if got := texts(t, o); got != "a" {
		t.Fatalf("history %q, want b gone with its newer copy", got)
	}
}

// TestOnceWhileWaiting: a copy the server has yet to number is listed once
// too, in place of the older one it will remove.
func TestOnceWhileWaiting(t *testing.T) {
	s := open(t)
	apply(t, s, copyAt(1, t0.Add(-2*time.Minute), "a"), copyAt(2, t0.Add(-time.Minute), "b"))
	f, err := s.Add(ctx, wire.NewCopy("text", []byte("a")), true)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := texts(t, s), "("+f.Ref+") b"; got != want {
		t.Fatalf("history %q, want %q", got, want)
	}
	f.Type, f.Seq = wire.Event, 3
	apply(t, s, f)
	if got := texts(t, s); got != "a b" {
		t.Fatalf("numbered: history %q, want a b", got)
	}
	if e, _, _ := s.Newest(ctx, true); e.Seq != 3 {
		t.Errorf("Newest = %d, want 3", e.Seq)
	}
}

// TestOnceInAnOlderHistory: a history kept before copies were held once
// holds each once when it is opened.
func TestOnceInAnOlderHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, text := range []string{"a", "b", "a", "a"} {
		if _, err := s.db.Exec(`INSERT INTO events (seq, kind, time, formats, data) VALUES (?, 'copy', ?, ?, ?)`,
			i+1, t0.Add(time.Duration(i)*time.Second).UnixMilli(), formatsText(wire.NewCopy("text", nil).Formats), text); err != nil {
			t.Fatal(err)
		}
	}
	// as a history of version 1 was: without what later migrations add
	if _, err := s.db.Exec(`ALTER TABLE outbox DROP COLUMN origin; PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.now = func() time.Time { return t0 }
	list, _ := s.List(ctx, 10)
	var seqs []uint64
	for _, e := range list {
		seqs = append(seqs, e.Seq)
	}
	if fmt.Sprint(seqs) != "[4 2]" {
		t.Fatalf("copies %v, want [4 2]: the last a, and b", seqs)
	}
}

func TestBounds(t *testing.T) {
	s := open(t)
	for i := range uint64(MaxCopies + 5) {
		apply(t, s, copyAt(i+1, t0.Add(time.Duration(i)*time.Second), fmt.Sprint("copy ", i))) // each its own, or they are one
	}
	list, _ := s.List(ctx, 1000)
	if len(list) != MaxCopies || list[len(list)-1].Seq != 6 {
		t.Fatalf("%d copies, the oldest %d; want %d, from 6", len(list), list[len(list)-1].Seq, MaxCopies)
	}

	s = open(t)
	for i := range uint64(3) {
		big := make([]byte, MaxBytes/2+1)
		big[0] = byte(i)
		f := wire.NewCopy("image/png", big)
		f.Type, f.Seq, f.Time = wire.Event, i+1, t0.Add(time.Duration(i)*time.Second).UnixMilli()
		apply(t, s, f)
	}
	if list, _ := s.List(ctx, 10); len(list) != 1 || list[0].Seq != 3 {
		t.Fatalf("%+v, want the newest only, within %d bytes", list, MaxBytes)
	}

	s = open(t)
	apply(t, s, copyAt(1, t0.Add(-MaxAge-time.Minute), "too old"), copyAt(2, t0, "new"))
	if got := texts(t, s); got != "new" {
		t.Fatalf("history %q, want the copy past %v gone", got, MaxAge)
	}
}

func TestFileIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Unix permissions")
	}
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want 0600", fi.Mode().Perm())
	}
	if s, err = Open(path); err != nil {
		t.Fatalf("reopening: %v", err)
	}
	s.Close()
}

// TestPreviews: the history list can carry the start of each text, for the
// web page and mg history to show; an image has none.
func TestPreviews(t *testing.T) {
	s := open(t)
	png := wire.NewCopy("image/png", []byte("\x89PNG not really"))
	png.Type, png.Seq, png.Time = wire.Event, 2, t0.UnixMilli()
	apply(t, s, copyAt(1, t0.Add(-time.Minute), "a long text, cut short"), png)
	s.Add(ctx, wire.NewCopy("text", []byte("waiting")), false)

	list, err := s.Previews(ctx, 10, 6)
	if err != nil {
		t.Fatal(err)
	}
	// a copy waiting to be numbered has its preview too; an image none
	if len(list) != 3 || string(list[0].Data) != "waitin" || list[1].Data != nil || string(list[2].Data) != "a long" {
		t.Fatalf("previews %+v", list)
	}
	if list[2].Size() != len("a long text, cut short") {
		t.Fatalf("a preview changed the size: %d", list[2].Size())
	}
	if plain, _ := s.List(ctx, 10); plain[2].Data != nil {
		t.Fatal("List carries bytes")
	}
}

// TestWritesAtOnce: a copy made on the device while events arrive from the
// server, as happens, waits for the other write rather than failing: a
// write that fails loses an event, or the copy.
func TestWritesAtOnce(t *testing.T) {
	s := open(t)
	const n = 200
	errs := make(chan error, 2*n)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range uint64(n) {
			if _, _, err := s.Apply(ctx, copyAt(i+1, t0.Add(time.Duration(i)*time.Millisecond), fmt.Sprint("from afar ", i))); err != nil {
				errs <- fmt.Errorf("Apply(%d): %w", i+1, err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := range n {
			if _, err := s.Add(ctx, wire.NewCopy("text", []byte(fmt.Sprint("here ", i))), false); err != nil {
				errs <- fmt.Errorf("Add(%d): %w", i, err)
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

// TestWaitingCopy: a copy still in the outbox is on this device, bytes and
// all, so it can be shown, put back on the clipboard, or taken back before
// the server has it.
func TestWaitingCopy(t *testing.T) {
	s := open(t)
	sent, err := s.Add(ctx, wire.NewCopy("image/png", []byte("a picture")), true)
	if err != nil {
		t.Fatal(err)
	}
	e, err := s.Waiting(ctx, sent.Ref)
	if err != nil || !e.Waiting() || e.Ref != sent.Ref || string(e.Data) != "a picture" || e.Formats[0].MIME != "image/png" {
		t.Fatalf("Waiting = %+v, %v", e, err)
	}
	if _, err := s.Waiting(ctx, "no-such-ref"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Waiting(unknown) = %v, want ErrNotFound", err)
	}
	if err := s.Forget(ctx, sent.Ref); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Waiting(ctx, sent.Ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Waiting after taking it back = %v, want ErrNotFound", err)
	}
	if out, _ := s.Outbox(ctx); len(out) != 0 {
		t.Fatalf("the outbox still has %+v", out)
	}
	if list, _ := s.List(ctx, 10); len(list) != 0 {
		t.Fatalf("the history still lists %+v", list)
	}
}
