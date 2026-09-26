// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"testing"
)

func TestNewDaemonID(t *testing.T) {
	if id := NewDaemon().ID; id == "" {
		t.Fatal("daemon has an empty ID")
	}
}
