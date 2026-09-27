// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package device

import (
	"bytes"
	"testing"

	"changkun.de/x/midgard/internal/e2e"
	"changkun.de/x/midgard/internal/wire"
)

// TestSealAndOpen: what the device seals, it opens; what it cannot open,
// or what comes in the clear after its person's key was made, it keeps as
// nothing (specs/redesign.md §11).
func TestSealAndOpen(t *testing.T) {
	key, _ := e2e.NewKey()
	copy := wire.NewCopy("text", []byte("hello"))
	copy.Ref = "r1"

	sealed, err := seal(key, copy)
	if err != nil || sealed.Kid != key.ID() || bytes.Contains(sealed.Payload, []byte("hello")) {
		t.Fatalf("seal = %+v, %v", sealed, err)
	}
	if _, err := sealed.Marshal(); err != nil {
		t.Fatalf("a sealed copy does not go on the wire: %v", err)
	}
	ev := sealed
	ev.Type, ev.Seq = wire.Event, 7
	if got := open(key, 0, ev); got.Kind != wire.KindCopy || string(got.Payload) != "hello" || got.Kid != "" {
		t.Fatalf("open = %+v", got)
	}

	other, _ := e2e.NewKey()
	changed := ev
	changed.Payload = append([]byte(nil), ev.Payload...)
	changed.Payload[len(changed.Payload)-1] ^= 1
	clear := copy
	clear.Type, clear.Seq = wire.Event, 7
	for name, tt := range map[string]struct {
		key   *e2e.Key
		since uint64
		f     wire.Frame
		kept  bool
	}{
		"sealed with another key":        {other, 0, ev, false},
		"changed":                        {key, 0, changed, false},
		"in the clear, after the key":    {key, 6, clear, false},
		"in the clear, before the key":   {key, 7, clear, true},
		"in the clear, and no key yet":   {nil, 0, clear, true},
		"sealed, and this device no key": {nil, 0, ev, false},
	} {
		got := open(tt.key, tt.since, tt.f)
		if kept := got.Kind == wire.KindCopy; kept != tt.kept {
			t.Errorf("%s: kept %v, want %v", name, kept, tt.kept)
		}
		if !tt.kept && (got.Kind != wire.KindVoid || got.Seq != 7 || got.Ref != "r1" || got.Payload != nil) {
			t.Errorf("%s: %+v, want a void numbered 7 that keeps its ref, for the outbox", name, got)
		}
	}

	// a preview is sealed as one, and nothing else is sealed
	preview := copy
	preview.Type, preview.Bare, preview.Payload = wire.Have, true, []byte("hel")
	p, _ := seal(key, preview)
	if got, err := key.Open(e2e.Preview, preview.Formats, p.Payload); err != nil || string(got) != "hel" {
		t.Errorf("a preview opened to %q, %v", got, err)
	}
	del := wire.Frame{Envelope: wire.Envelope{Type: wire.Delete, Kind: wire.KindDelete, Target: 3}}
	if d, _ := seal(key, del); d.Kid != "" {
		t.Error("a delete was sealed")
	}
	if n, _ := seal(nil, copy); n.Kid != "" || string(n.Payload) != "hello" {
		t.Error("a device without a key sealed")
	}
}
