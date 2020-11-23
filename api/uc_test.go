// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package api

import (
	"bytes"
	"testing"

	"golang.design/x/midgard/clipboard"
)

func TestUniversalClipboard(t *testing.T) {
	data := []byte("hello")
	uc0.put(clipboard.DataTypePlainText, data)

	ret := uc0.get(clipboard.DataTypePlainText)
	if bytes.Compare(data, ret) != 0 {
		t.Fatalf("failed to put data into ub.")
	}

	ret = uc0.get(clipboard.DataTypeImagePNG)
	if bytes.Compare(data, ret) == 0 {
		t.Fatalf("unexpected read from ub, want blank, got %v", string(ret))
	}

	tt, ret := uc0.read()

	if tt != clipboard.DataTypePlainText {
		t.Fatalf("incorrect data type")
	}
	if bytes.Compare(data, ret) != 0 {
		t.Fatalf("incorrect data from clipboard")
	}

	t.Log(string(data))
}
