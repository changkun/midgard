// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package e2e

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"changkun.de/x/midgard/internal/wire"
)

var text = []wire.Format{{MIME: "text", Size: 5}}

func TestSealOpen(t *testing.T) {
	k, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := k.Seal(Copy, text, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sealed) != 5+Overhead || bytes.Contains(sealed, []byte("hello")) {
		t.Fatalf("sealed %d bytes, want %d, and not the text", len(sealed), 5+Overhead)
	}
	if got, err := k.Open(Copy, text, sealed); err != nil || string(got) != "hello" {
		t.Fatalf("Open = %q, %v", got, err)
	}
	again, _ := k.Seal(Copy, text, []byte("hello"))
	if bytes.Equal(sealed, again) {
		t.Error("the same copy sealed twice is the same bytes: the nonce is not random")
	}

	// nothing else opens it, and nothing opens it changed
	other, _ := NewKey()
	flipped := append([]byte(nil), sealed...)
	flipped[len(flipped)-1] ^= 1
	for name, open := range map[string]func() ([]byte, error){
		"another key":        func() ([]byte, error) { return other.Open(Copy, text, sealed) },
		"as a preview":       func() ([]byte, error) { return k.Open(Preview, text, sealed) },
		"as another type":    func() ([]byte, error) { return k.Open(Copy, []wire.Format{{MIME: "image/png", Size: 5}}, sealed) },
		"as another size":    func() ([]byte, error) { return k.Open(Copy, []wire.Format{{MIME: "text", Size: 4}}, sealed) },
		"changed":            func() ([]byte, error) { return k.Open(Copy, text, flipped) },
		"cut short":          func() ([]byte, error) { return k.Open(Copy, text, sealed[:Overhead-1]) },
		"in the clear":       func() ([]byte, error) { return k.Open(Copy, text, []byte("hello")) },
		"of another version": func() ([]byte, error) { return k.Open(Copy, text, append([]byte{2}, sealed[1:]...)) },
	} {
		if got, err := open(); !errors.Is(err, ErrOpen) {
			t.Errorf("%s: Open = %q, %v, want ErrOpen", name, got, err)
		}
	}
}

func TestKey(t *testing.T) {
	k, _ := NewKey()
	again, err := KeyFrom(k.Raw())
	if err != nil || again.ID() != k.ID() {
		t.Fatalf("KeyFrom(Raw()) = %v, %v", again, err)
	}
	if len(k.ID()) != 16 || strings.Trim(k.ID(), "0123456789abcdef") != "" {
		t.Errorf("ID = %q, want 16 hex digits", k.ID())
	}
	if other, _ := NewKey(); other.ID() == k.ID() {
		t.Error("two keys have one id")
	}
	if _, err := KeyFrom(make([]byte, 16)); err == nil {
		t.Error("KeyFrom took a 16-byte key")
	}
	raw := k.Raw()
	raw[0] ^= 1
	if k.Raw()[0] == raw[0] {
		t.Error("Raw gave the key's own bytes, to be changed")
	}
}

// TestVector: Go seals as WebCrypto does, which the web page uses: the
// expected bytes were made by crypto.subtle (Node's), from the same key,
// nonce, additional data and text, not by this package.
func TestVector(t *testing.T) {
	raw, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	k, err := KeyFrom(raw)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "f0f5111059cbd60c"
	if k.ID() != kid {
		t.Fatalf("ID = %q, want %q", k.ID(), kid)
	}
	nonce, _ := hex.DecodeString("a0a1a2a3a4a5a6a7a8a9aaab")
	formats := []wire.Format{{MIME: "text", Size: 12}}
	if got, want := string(AdditionalData(kid, Copy, formats)), "midgard/e2e/v1\n"+kid+"\ncopy\ntext:12"; got != want {
		t.Fatalf("AdditionalData = %q, want %q", got, want)
	}
	sealed := k.seal(nonce, Copy, formats, []byte("hello, world"))
	const want = "01a0a1a2a3a4a5a6a7a8a9aaab8e7d10412ae722c80d17ebb75425546fdf652a3096f62a04941d307c"
	if got := hex.EncodeToString(sealed); got != want {
		t.Fatalf("sealed = %s, want %s, as WebCrypto seals it", got, want)
	}
	if got, err := k.Open(Copy, formats, sealed); err != nil || string(got) != "hello, world" {
		t.Fatalf("Open = %q, %v", got, err)
	}
}
