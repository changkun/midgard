// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"testing"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/signin"
)

// TestExitCodes: a script tells what went wrong by the code, however the
// error was wrapped on its way.
func TestExitCodes(t *testing.T) {
	for err, want := range map[error]int{
		nil:                                     exitOK,
		signin.ErrSignedOut:                     exitSignedIn,
		fmt.Errorf("x: %w", client.ErrNoDevice): exitOffline,
		client.ErrNotFound:                      exitNotFound,
		client.ErrNotPaired:                     exitNotPaired,
		errors.New("the disk is full"):          exitFailed,
	} {
		if got := exitCode(err); got != want {
			t.Errorf("exitCode(%v) = %d, want %d", err, got, want)
		}
	}
}
