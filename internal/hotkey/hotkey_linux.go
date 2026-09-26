// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

//go:build linux && cgo

package hotkey

import "golang.design/x/hotkey"

// getModifiers is Ctrl+Super. Mod2 is NumLock on most keyboards; it used to
// be listed here too, so the hotkey fired only while NumLock was on. The
// hotkey package already listens with every NumLock and CapsLock state.
func getModifiers() []hotkey.Modifier {
	return []hotkey.Modifier{
		hotkey.ModCtrl,
		hotkey.Mod4,
	}
}

func getKey() hotkey.Key {
	return hotkey.KeyS
}
