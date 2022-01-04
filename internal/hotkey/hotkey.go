// Copyright 2022 Changkun Ou. All rights reserved.
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
	hk := hotkey.New(getModifiers(), getKey())
	if err := hk.Register(); err != nil {
		log.Printf("Hotkey registration failed: %v", err)
		return
	}
	log.Println("hotkey registration success.")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hk.Keydown():
				fn()
				runtime.KeepAlive(hk)
			}
		}
	}()
}
