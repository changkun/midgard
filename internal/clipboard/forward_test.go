// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard

import (
	"context"
	"testing"

	"golang.design/x/clipboard"
)

// TestForwardDropsSensitive: a password copied from a password manager is
// marked, and must not be synced; everything else is passed on in order.
func TestForwardDropsSensitive(t *testing.T) {
	src := make(chan clipboard.Data, 3)
	src <- clipboard.Data{Format: clipboard.FmtText, Bytes: []byte("before")}
	src <- clipboard.Data{Format: clipboard.FmtText, Bytes: []byte("hunter2"), Sensitive: true}
	src <- clipboard.Data{Format: clipboard.FmtText, Bytes: []byte("after")}
	close(src)

	out := make(chan []byte)
	go forward(context.Background(), src, out)

	var got []string
	for b := range out {
		got = append(got, string(b))
	}
	if len(got) != 2 || got[0] != "before" || got[1] != "after" {
		t.Fatalf("forwarded %q, want [before after]", got)
	}
}

// TestForwardDropsImagesThatAreNot: xclip answers a request for image/png
// with the text it holds, and that text was synced as an image.
func TestForwardDropsImagesThatAreNot(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n and the rest")
	src := make(chan clipboard.Data, 2)
	src <- clipboard.Data{Format: clipboard.FmtImage, Bytes: []byte("hello, text answered for image/png")}
	src <- clipboard.Data{Format: clipboard.FmtImage, Bytes: png}
	close(src)

	out := make(chan []byte)
	go forward(context.Background(), src, out)
	var got [][]byte
	for b := range out {
		got = append(got, b)
	}
	if len(got) != 1 || string(got[0]) != string(png) {
		t.Fatalf("forwarded %q, want the PNG alone", got)
	}
}
