// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard_test

import (
	"bytes"
	"testing"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
)

func TestUniversalClipboard(t *testing.T) {
	buf := utils.StringToBytes("hello")
	clipboard.Universal.Put(types.ClipboardDataTypePlainText, buf)

	got := clipboard.Universal.Get(types.ClipboardDataTypePlainText)
	if bytes.Compare(buf, got) != 0 {
		t.Fatalf("failed to put data into ub.")
	}

	got = clipboard.Universal.Get(types.ClipboardDataTypeImagePNG)
	if bytes.Compare(buf, got) == 0 {
		t.Fatalf("unexpected read from ub, want blank, got %v", utils.BytesToString(got))
	}

	tt, got := clipboard.Universal.Read()

	if tt != types.ClipboardDataTypePlainText {
		t.Fatalf("incorrect data type")
	}
	if bytes.Compare(buf, got) != 0 {
		t.Fatalf("incorrect data from clipboard")
	}

	t.Log(utils.BytesToString(buf))
}
