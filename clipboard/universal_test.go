// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package clipboard_test

import (
	"bytes"
	"testing"

	"golang.design/x/midgard/clipboard"
	"golang.design/x/midgard/types"
)

func TestUniversalClipboard(t *testing.T) {
	buf := []byte("hello")
	clipboard.Universal.Put(types.ClipboardDataTypePlainText, buf)

	got := clipboard.Universal.Get(types.ClipboardDataTypePlainText)
	if bytes.Compare(buf, got) != 0 {
		t.Fatalf("failed to put data into ub.")
	}

	got = clipboard.Universal.Get(types.ClipboardDataTypeImagePNG)
	if bytes.Compare(buf, got) == 0 {
		t.Fatalf("unexpected read from ub, want blank, got %v", string(got))
	}

	tt, got := clipboard.Universal.Read()

	if tt != types.ClipboardDataTypePlainText {
		t.Fatalf("incorrect data type")
	}
	if bytes.Compare(buf, got) != 0 {
		t.Fatalf("incorrect data from clipboard")
	}

	t.Log(string(buf))
}
