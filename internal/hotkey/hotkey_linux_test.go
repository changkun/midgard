// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

//go:build linux && cgo

package hotkey

import (
	"testing"

	"golang.design/x/hotkey"
)

// TestHotkeyIgnoresNumLock: a lock key in the combination makes the hotkey
// fire only while that lock is on.
func TestHotkeyIgnoresNumLock(t *testing.T) {
	for _, m := range getModifiers() {
		if m == hotkey.Mod2 {
			t.Fatal("the hotkey needs Mod2, NumLock, to be on")
		}
	}
}
