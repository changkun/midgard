// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package utils_test

import (
	"strings"
	"testing"

	"changkun.de/x/midgard/internal/utils"
)

func TestNewUUID(t *testing.T) {
	id, err := utils.NewUUID()
	if err != nil {
		t.Fatal("cannot allocate a new uuid")
	}

	t.Log(id)
}
func TestNewUUIDShort(t *testing.T) {
	id, err := utils.NewUUIDShort()
	if err != nil {
		t.Fatal("cannot allocate a new uuid")
	}

	t.Log(id)
}

// The short form is persisted: it is the path of every URL midgard has ever
// allocated. Its length and alphabet must not drift with an encoder change.
func TestNewUUIDShortForm(t *testing.T) {
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	seen := map[string]bool{}
	for range 100 {
		id, err := utils.NewUUIDShort()
		if err != nil {
			t.Fatalf("cannot allocate a new uuid: %v", err)
		}
		// 128 bits in base 57 is at most ceil(128/log2(57)) = 22 digits,
		// fewer when the leading digits are zero; the encoder pads to 13.
		if len(id) < 13 || len(id) > 22 {
			t.Fatalf("short uuid %q has length %d, want 13..22", id, len(id))
		}
		for _, r := range id {
			if !strings.ContainsRune(alphabet, r) {
				t.Fatalf("short uuid %q contains %q, which is outside the alphabet", id, r)
			}
		}
		if seen[id] {
			t.Fatalf("short uuid %q was allocated twice", id)
		}
		seen[id] = true
	}
}
