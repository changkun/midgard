// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package e2e seals a person's copies with the key their devices share, so
// the server passes bytes it cannot read (specs/redesign.md §11).
//
// A key is 256 random bits, made by a person's first device and given to
// the others by pairing (pair.go). Sealing is AES-256-GCM, which the web
// page has too, in WebCrypto: the sealed bytes are a version byte, a random
// 96-bit nonce, then the ciphertext and its tag. The additional data binds
// them to the key's id, to what they are (a copy, or a preview of one), and
// to the copy's formats, so the server cannot pass one off as another. It
// can still replay what it has seen, and reorder it: it numbers events.
package e2e

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"changkun.de/x/midgard/internal/wire"
)

// KeySize is the size of a key, in bytes.
const KeySize = 32

// version is the first byte of what Seal makes, for a later way of sealing
// to be told from this one.
const version = 1

// Overhead is how many bytes sealing adds: the version, the nonce, the tag.
const Overhead = 1 + 12 + 16

// ErrOpen is what opening says of bytes not sealed with this key, sealed as
// something else, or changed since.
var ErrOpen = errors.New("e2e: cannot open: not sealed with this key, or changed")

// Key is a person's key.
type Key struct {
	raw  []byte
	id   string
	aead cipher.AEAD
}

// NewKey makes a key, for a person's first device.
func NewKey() (*Key, error) {
	raw := make([]byte, KeySize)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	return KeyFrom(raw)
}

// KeyFrom is the key raw holds, as Raw gave it.
func KeyFrom(raw []byte) (*Key, error) {
	if len(raw) != KeySize {
		return nil, fmt.Errorf("e2e: a key is %d bytes, not %d", KeySize, len(raw))
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append([]byte("midgard/key/v1 id\n"), raw...))
	return &Key{raw: append([]byte(nil), raw...), id: hex.EncodeToString(sum[:8]), aead: aead}, nil
}

// ID is the key's id, its kid: 16 hex digits, which the server keeps to know
// whether a device has its person's key. It says nothing of the key.
func (k *Key) ID() string { return k.id }

// Raw is the key's bytes, to keep on the device, and to pair another.
func (k *Key) Raw() []byte { return append([]byte(nil), k.raw...) }

// Kind is what sealed bytes are, bound into them.
type Kind string

const (
	// Copy is a copy's bytes: its formats, one after another.
	Copy Kind = "copy"
	// Preview is the start of a copy's text, as a list of copies shows it.
	Preview Kind = "preview"
)

// Seal seals plain, the bytes of kind with formats.
func (k *Key) Seal(kind Kind, formats []wire.Format, plain []byte) ([]byte, error) {
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return k.seal(nonce, kind, formats, plain), nil
}

func (k *Key) seal(nonce []byte, kind Kind, formats []wire.Format, plain []byte) []byte {
	out := make([]byte, 0, Overhead+len(plain))
	out = append(out, version)
	out = append(out, nonce...)
	return k.aead.Seal(out, nonce, plain, AdditionalData(k.id, kind, formats))
}

// Open opens what Seal sealed as kind with formats.
func (k *Key) Open(kind Kind, formats []wire.Format, sealed []byte) ([]byte, error) {
	n := k.aead.NonceSize()
	if len(sealed) < 1+n+k.aead.Overhead() || sealed[0] != version {
		return nil, ErrOpen
	}
	plain, err := k.aead.Open(nil, sealed[1:1+n], sealed[1+n:], AdditionalData(k.id, kind, formats))
	if err != nil {
		return nil, ErrOpen
	}
	return plain, nil
}

// AdditionalData is what a seal is bound to, as lines: its scheme, the key's
// id, the kind, and the formats as "mime:size", comma-separated. The web
// page makes the same bytes to open and seal.
func AdditionalData(kid string, kind Kind, formats []wire.Format) []byte {
	parts := make([]string, len(formats))
	for i, f := range formats {
		parts[i] = f.MIME + ":" + strconv.Itoa(f.Size)
	}
	return []byte("midgard/e2e/v1\n" + kid + "\n" + string(kind) + "\n" + strings.Join(parts, ","))
}
