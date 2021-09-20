// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package office_test

import (
	"testing"

	"changkun.de/x/midgard/internal/office"
)

func TestIsScreenLocked(t *testing.T) {
	locked, err := office.IsScreenLocked()
	if err != nil {
		t.Fatalf("check screen status failed: %v", err)
	}

	if locked {
		t.Fatalf("check screen status failed: screen is locked")
	}
}

func TestIsInMeeting(t *testing.T) {
	meeting, err := office.IsInMeeting()
	if err != nil {
		t.Fatalf("check meeting status failed: %v", err)
	}

	if meeting {
		t.Fatalf("check meeting status failed: currently in a meeting")
	}
}
