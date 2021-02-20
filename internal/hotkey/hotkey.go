// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package hotkey

import "context"

// Handle registers an application global hotkey to the system,
// and returns a channel that will signal if the hotkey is triggered.
//
// No customization for the hotkey, the hotkey is always:
// Linux: Ctrl+Mod4+s
// macOS: Ctrl+Option+s
// Windows: Unsupported
func Handle(ctx context.Context, fn func()) {
	go handle(ctx, fn)
}
