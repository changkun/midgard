// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package utils_test

import (
	"bytes"
	"testing"

	"changkun.de/x/midgard/internal/utils"
)

func TestBytesString(t *testing.T) {
	s := utils.BytesToString(nil)
	if s != "" {
		t.Fatalf("failed to convert nil bytes")
	}

	b := utils.StringToBytes("")
	if b != nil {
		t.Fatalf("failed to convert empty string")
	}
}

func TestStringToBytes(t *testing.T) {
	for _, want := range []string{
		"a",
		"changkun.de/x/midgard",
		"\x00\x01\xff",
		"日本語",
	} {
		got := utils.StringToBytes(want)
		if !bytes.Equal(got, []byte(want)) {
			t.Fatalf("StringToBytes(%q) = %v, want %v", want, got, []byte(want))
		}
		if len(got) != len(want) || cap(got) != len(want) {
			t.Fatalf("StringToBytes(%q) has len %d cap %d, want both %d",
				want, len(got), cap(got), len(want))
		}
	}
}

func TestBytesToString(t *testing.T) {
	for _, want := range []string{
		"a",
		"changkun.de/x/midgard",
		"\x00\x01\xff",
		"日本語",
	} {
		if got := utils.BytesToString([]byte(want)); got != want {
			t.Fatalf("BytesToString = %q, want %q", got, want)
		}
	}
}

// The conversions must not copy: both directions alias the input. A copy
// would still pass the equality tests above but lose the point of the
// package, so measure the allocations directly.
func TestStrBytesDoNotAllocate(t *testing.T) {
	s := "changkun.de/x/midgard"
	b := []byte(s)

	if n := testing.AllocsPerRun(100, func() {
		_ = utils.StringToBytes(s)
	}); n != 0 {
		t.Fatalf("StringToBytes allocates %v times per run, want 0", n)
	}
	if n := testing.AllocsPerRun(100, func() {
		_ = utils.BytesToString(b)
	}); n != 0 {
		t.Fatalf("BytesToString allocates %v times per run, want 0", n)
	}
}
