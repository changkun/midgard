// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package e2e

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// Pairing gives the key to a new device (specs/redesign.md §11). A paired
// device makes a Code and shows it; it seals the key under the code, and
// leaves the box in a mailbox on the server, named by the code's Mailbox.
// The new device, given the code, fetches the box and opens it. The server
// holds a box it cannot open without the code: 128 random bits, too many to
// guess.

// codeBits is how much of a code is random.
const codeBits = 128

// alphabet is Crockford's base32: no I, L, O or U, which a person would
// misread or mistype.
const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// CodeLen is how many characters a code has: 128 bits, 5 to a character.
const CodeLen = (codeBits + 4) / 5

// ErrCode is what ParseCode says of what is not a pairing code.
var ErrCode = errors.New("e2e: not a pairing code")

// Code is a pairing code, as CodeLen characters of the alphabet.
type Code struct{ secret []byte }

// NewCode makes a code, for a paired device to show.
func NewCode() (Code, error) {
	secret := make([]byte, codeBits/8)
	if _, err := rand.Read(secret); err != nil {
		return Code{}, err
	}
	return Code{secret}, nil
}

// String is the code as a person types it: groups of four, dashed.
func (c Code) String() string {
	s := c.plain()
	var b strings.Builder
	for i := 0; i < len(s); i += 4 {
		if i > 0 {
			b.WriteByte('-')
		}
		b.WriteString(s[i:min(i+4, len(s))])
	}
	return b.String()
}

// plain is the code's characters, undashed, as a link carries them.
func (c Code) plain() string {
	// 128 bits as 26 characters of 5 bits, the first taking the top 3
	out := make([]byte, CodeLen)
	var acc uint64
	var n, j uint
	for i := len(c.secret) - 1; i >= 0; i-- { // from the low end
		acc |= uint64(c.secret[i]) << n
		n += 8
		for n >= 5 {
			out[CodeLen-1-int(j)] = alphabet[acc&31]
			acc >>= 5
			n -= 5
			j++
		}
	}
	if j < CodeLen {
		out[CodeLen-1-int(j)] = alphabet[acc&31]
	}
	return string(out)
}

// Link is the code as the part of a pairing link after its #, which a
// browser does not send to the server.
func (c Code) Link() string { return "pair=" + c.plain() }

// ParseCode reads a code as a person gives it: in either case, with dashes
// or spaces, and O, I and L taken for 0, 1 and 1, as Crockford reads them.
func ParseCode(s string) (Code, error) {
	var chars []byte
	for _, r := range strings.ToUpper(s) {
		switch {
		case r == '-' || r == ' ':
			continue
		case r == 'O':
			r = '0'
		case r == 'I' || r == 'L':
			r = '1'
		}
		if r > 127 || strings.IndexRune(alphabet, r) < 0 {
			return Code{}, ErrCode
		}
		chars = append(chars, byte(r))
	}
	if len(chars) != CodeLen {
		return Code{}, ErrCode
	}
	secret := make([]byte, codeBits/8)
	var acc uint64
	var n uint
	k := len(secret) - 1
	for i := CodeLen - 1; i >= 0; i-- {
		acc |= uint64(strings.IndexByte(alphabet, chars[i])) << n
		n += 5
		for n >= 8 && k >= 0 {
			secret[k] = byte(acc)
			acc >>= 8
			n -= 8
			k--
		}
	}
	if acc != 0 { // the first character holds only 3 bits
		return Code{}, ErrCode
	}
	return Code{secret}, nil
}

// Mailbox is where the box for this code waits on the server: a hash of the
// code, from which the code cannot be had.
func (c Code) Mailbox() string {
	sum := sha256.Sum256(append([]byte("midgard/pair/v1 mailbox\n"), c.secret...))
	return hex.EncodeToString(sum[:16])
}

// Seal seals the key into a box that only this code opens.
func (c Code) Seal(k *Key) ([]byte, error) {
	aead, err := c.aead()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte{version}, nonce...)
	return aead.Seal(out, nonce, k.raw, c.boxData()), nil
}

// Open opens a box this code sealed, to the key.
func (c Code) Open(box []byte) (*Key, error) {
	aead, err := c.aead()
	if err != nil {
		return nil, err
	}
	n := aead.NonceSize()
	if len(box) < 1+n+aead.Overhead() || box[0] != version {
		return nil, ErrOpen
	}
	raw, err := aead.Open(nil, box[1:1+n], box[1+n:], c.boxData())
	if err != nil {
		return nil, ErrOpen
	}
	return KeyFrom(raw)
}

func (c Code) aead() (cipher.AEAD, error) {
	key, err := hkdf.Key(sha256.New, c.secret, nil, "midgard/pair/v1 key", KeySize)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// boxData binds a box to its mailbox, so one left in another cannot pass.
func (c Code) boxData() []byte { return []byte("midgard/pair/v1\n" + c.Mailbox()) }
