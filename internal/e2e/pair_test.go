// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package e2e

import (
	"errors"
	"strings"
	"testing"
)

func TestCode(t *testing.T) {
	c, err := NewCode()
	if err != nil {
		t.Fatal(err)
	}
	s := c.String()
	if plain := strings.ReplaceAll(s, "-", ""); len(plain) != CodeLen || strings.Trim(plain, alphabet) != "" {
		t.Fatalf("code %q: want %d characters of %s", s, CodeLen, alphabet)
	}
	if strings.Count(s, "-") != (CodeLen-1)/4 {
		t.Errorf("code %q: want it grouped by four", s)
	}
	if other, _ := NewCode(); other.String() == s {
		t.Error("two codes are one")
	}

	// a person gives it back however they give it
	plain := strings.ReplaceAll(s, "-", "")
	for _, given := range []string{
		s,
		plain,
		strings.ToLower(s),
		strings.ReplaceAll(s, "-", " "),
		strings.NewReplacer("0", "O", "1", "I").Replace(plain),
		strings.ReplaceAll(strings.ToLower(plain), "1", "l"),
		strings.TrimPrefix(c.Link(), "pair="),
	} {
		got, err := ParseCode(given)
		if err != nil || got.String() != s || got.Mailbox() != c.Mailbox() {
			t.Errorf("ParseCode(%q) = %v, %v, want %s", given, got, err, s)
		}
	}
	for _, bad := range []string{"", plain[1:], plain + "0", plain[:CodeLen-1] + "U", "Z" + plain[1:]} {
		if _, err := ParseCode(bad); !errors.Is(err, ErrCode) {
			t.Errorf("ParseCode(%q) = %v, want ErrCode", bad, err)
		}
	}
}

func TestPairing(t *testing.T) {
	k, _ := NewKey()
	c, _ := NewCode()
	box, err := c.Seal(k)
	if err != nil {
		t.Fatal(err)
	}
	// the new device, given the code as the paired one showed it
	given, _ := ParseCode(c.String())
	got, err := given.Open(box)
	if err != nil || got.ID() != k.ID() {
		t.Fatalf("Open = %v, %v, want the key %s", got, err, k.ID())
	}
	if len(c.Mailbox()) != 32 || strings.Contains(c.Mailbox(), strings.ToLower(strings.ReplaceAll(c.String(), "-", ""))) {
		t.Errorf("Mailbox = %q, want 32 hex digits, not the code", c.Mailbox())
	}

	other, _ := NewCode()
	if other.Mailbox() == c.Mailbox() {
		t.Error("two codes share a mailbox")
	}
	if _, err := other.Open(box); !errors.Is(err, ErrOpen) {
		t.Errorf("another code opened the box: %v", err)
	}
	box[len(box)-1] ^= 1
	if _, err := c.Open(box); !errors.Is(err, ErrOpen) {
		t.Errorf("a changed box opened: %v", err)
	}
}
