// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"os"
	"testing"

	"changkun.de/x/midgard/testdata"
)

func TestMain(m *testing.M) {
	testdata.UseConfig()
	os.Exit(m.Run())
}
