// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// +build !(freebsd,linux,netbsd,openbsd,solaris,dragonfly,darwin)

package hotkey

import "context"

func handle(ctx context.Context, fn func()) { panic("unimplemented") }
