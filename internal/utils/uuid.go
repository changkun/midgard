// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strings"
)

// random function
var reader = rand.Reader

var nilUUID uuid // empty UUID, all zeros

// A uuid is a 128 bit (16 byte) Universal Unique IDentifier
// as defined in RFC 4122.
type uuid [16]byte

// String returns the string form of uuid
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
// or "" if uuid is invalid.
func (u uuid) String() string {
	var buf [36]byte
	encodeHex(buf[:], u)
	return string(buf[:])
}

// NewUUID creates a new uuid.
func NewUUID() (uuid, error) {
	var u uuid
	_, err := io.ReadFull(reader, u[:])
	if err != nil {
		return nilUUID, err
	}
	u[6] = (u[6] & 0x0f) | 0x40 // Version 4
	u[8] = (u[8] & 0x3f) | 0x80 // Variant is 10
	return u, nil
}

func encodeHex(dst []byte, u uuid) {
	hex.Encode(dst, u[:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], u[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], u[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], u[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:], u[10:])
}

// NewUUIDShort creates a new uuid and encodes it to a short form.
func NewUUIDShort() (string, error) {
	id, err := NewUUID()
	if err != nil {
		return "", err
	}
	return uuidEncoder.Encode(id), nil
}

// a is the default alphabet used.
const a = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

type alphabet struct {
	chars [57]string
	len   int64
}

func (a *alphabet) Length() int64 {
	return a.len
}

// Index returns the index of the first instance of t in the alphabet,
// or an error if t is not present.
func (a *alphabet) Index(t string) (int64, error) {
	for i, char := range a.chars {
		if char == t {
			return int64(i), nil
		}
	}
	return 0, fmt.Errorf("element '%v' is not part of the alphabet", t)
}

// newAlphabet removes duplicates and sort it to ensure reproducability.
func newAlphabet(s string) alphabet {
	abc := dedupe(strings.Split(s, ""))

	if len(abc) != 57 {
		panic("encoding alphabet is not 57-bytes long")
	}

	sort.Strings(abc)
	a := alphabet{
		len: int64(len(abc)),
	}
	copy(a.chars[:], abc)
	return a
}

// dudupe removes duplicate characters from s.
func dedupe(s []string) []string {
	var out []string
	m := make(map[string]bool)

	for _, char := range s {
		if _, ok := m[char]; !ok {
			m[char] = true
			out = append(out, char)
		}
	}

	return out
}

var uuidEncoder = &base57{newAlphabet(a)}

type base57 struct {
	// alphabet is the character set to construct the UUID from.
	alphabet alphabet
}

// Encode encodes uuid.UUID into a string using the least significant
// bits (LSB) first according to the alphabet. if the most significant
// bits (MSB) are 0, the string might be shorter.
func (b base57) Encode(u uuid) string {
	var num big.Int
	num.SetString(strings.Replace(u.String(), "-", "", 4), 16)

	// Calculate encoded length.
	factor := math.Log(float64(25)) / math.Log(float64(b.alphabet.Length()))
	length := math.Ceil(factor * float64(len(u)))

	return b.numToString(&num, int(length))
}

// numToString converts a number to string using the given alphabet.
func (b *base57) numToString(number *big.Int, padToLen int) string {
	var (
		out   string
		digit *big.Int
	)

	for number.Uint64() > 0 {
		number, digit = new(big.Int).DivMod(number,
			big.NewInt(b.alphabet.Length()), new(big.Int))
		out += b.alphabet.chars[digit.Int64()]
	}

	if padToLen > 0 {
		remainder := math.Max(float64(padToLen-len(out)), 0)
		out = out + strings.Repeat(b.alphabet.chars[0], int(remainder))
	}

	return out
}
