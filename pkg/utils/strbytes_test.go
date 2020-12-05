// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package utils_test

import (
	"testing"

	"changkun.de/x/midgard/pkg/utils"
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
