// Copyright 2020-2021 Changkun Ou. All rights reserved.
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
	clipboard.UniversalFor("test-owner").Write(types.MIMEPlainText, buf)

	got := clipboard.UniversalFor("test-owner").ReadAs(types.MIMEPlainText)
	if !bytes.Equal(buf, got) {
		t.Fatalf("failed to put data into ub.")
	}

	got = clipboard.UniversalFor("test-owner").ReadAs(types.MIMEImagePNG)
	if bytes.Equal(buf, got) {
		t.Fatalf("unexpected read from ub, want blank, got %v", utils.BytesToString(got))
	}

	tt, got := clipboard.UniversalFor("test-owner").Read()

	if tt != types.MIMEPlainText {
		t.Fatalf("incorrect data type")
	}
	if !bytes.Equal(buf, got) {
		t.Fatalf("incorrect data from clipboard")
	}

	t.Log(utils.BytesToString(buf))
}

// TestUniversalPerOwner: there is no shared clipboard; one person's copy is
// not another's to read.
func TestUniversalPerOwner(t *testing.T) {
	clipboard.UniversalFor("alice").Write(types.MIMEPlainText, []byte("alice's"))
	if got := clipboard.UniversalFor("bob").ReadAs(types.MIMEPlainText); bytes.Equal(got, []byte("alice's")) {
		t.Fatal("bob read alice's clipboard")
	}
	if got := clipboard.UniversalFor("alice").ReadAs(types.MIMEPlainText); !bytes.Equal(got, []byte("alice's")) {
		t.Fatalf("alice's clipboard = %q", got)
	}
}
