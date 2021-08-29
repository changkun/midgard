// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package utils_test

import (
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
