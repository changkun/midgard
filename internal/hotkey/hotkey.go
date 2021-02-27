// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package hotkey

import (
	"context"
	"log"
	"runtime"

	"golang.design/x/hotkey"
)

// Handle registers an application global hotkey to the system,
// and returns a channel that will signal if the hotkey is triggered.
//
// No customization for the hotkey, the hotkey is always:
// Linux: Ctrl+Mod4+s
// macOS: Ctrl+Option+s
// Windows: Unsupported
func Handle(ctx context.Context, fn func()) {
	hk, err := hotkey.Register(getModifiers(), hotkey.KeyS)
	if err != nil {
		log.Printf("Hotkey registration failed: %v", err)
		return
	}
	log.Println("hotkey registration success.")

	go func() {
		trigger := hk.Listen(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-trigger:
				fn()
				runtime.KeepAlive(hk)
			}
		}
	}()
}
