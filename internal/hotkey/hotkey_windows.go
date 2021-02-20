// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// +build windows

package hotkey

import (
	"context"
	"fmt"
)

func handle(ctx context.Context, fn func()) {
	fmt.Println("hotkey is unimplemented on windows")
}
