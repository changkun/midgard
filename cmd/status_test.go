// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"testing"

	"changkun.de/x/midgard/internal/types"
)

func TestConnected(t *testing.T) {
	devices := []types.Device{{Name: "laptop", Online: true}, {Name: "desktop"}}
	for host, want := range map[string]bool{
		"laptop":  true,
		"desktop": false, // known, but offline
		"lap":     false,
		"server":  false,
	} {
		if got := connected(devices, host); got != want {
			t.Errorf("connected(%q) = %v, want %v", host, got, want)
		}
	}
}
