// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

//go:build darwin && cgo

package hotkey

import "golang.design/x/hotkey"

func getModifiers() []hotkey.Modifier {
	return []hotkey.Modifier{
		hotkey.ModCtrl,
		hotkey.ModOption,
	}
}

func getKey() hotkey.Key {
	return hotkey.KeyS
}
