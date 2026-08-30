// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard_test

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"reflect"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
)

func TestLocalClipboardImage(t *testing.T) {
	data, err := os.ReadFile("testdata/gold.png")
	if err != nil {
		t.Fatalf("failed to read gold file, err: %v", err)
	}
	clipboard.Local.Write(types.MIMEImagePNG, data)

	tp, r := clipboard.Local.Read()
	if tp != types.MIMEImagePNG {
		t.Fatalf("failed to read as image, err: %v", err)
	}

	img1, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("write image is not png encoded: %v", err)
	}
	img2, err := png.Decode(bytes.NewReader(r))
	if err != nil {
		t.Fatalf("read image is not png encoded: %v", err)
	}

	w := img2.Bounds().Dx()
	h := img2.Bounds().Dy()

	incorrect := 0
	for i := range w {
		for j := range h {
			want := img1.At(i, j)
			got := img2.At(i, j)

			wantR, wantG, wantB, wantA := want.RGBA()
			gotR, gotG, gotB, gotA := got.RGBA()
			if wantR != gotR || wantG != gotG || wantB != gotB || wantA != gotA {
				t.Logf("read data from clipbaord is inconsistent with previous written data, pix: (%d,%d), got: %+v,%+v,%+v,%+v, want: %+v,%+v,%+v,%+v", i, j, gotR, gotG, gotB, gotA, wantR, wantG, wantB, wantA)
				incorrect++
			}
		}
	}

	// FIXME: it looks like windows can produce incorrect pixels when y == 0.
	// Needs more investigation.
	if incorrect > w {
		t.Fatalf("read data from clipboard contains too much inconsistent pixels to the previous written data, number of incorrect pixels: %v", incorrect)
	}
}

func TestLocalClipboardText(t *testing.T) {
	data := utils.StringToBytes("changkun.de/x/midgard")
	clipboard.Local.Write(types.MIMEPlainText, data)

	tp, r := clipboard.Local.Read()
	if !reflect.DeepEqual(r, data) ||
		!reflect.DeepEqual(tp, types.MIMEPlainText) {
		t.Fatalf("inconsistent read of a write, got: %s", utils.BytesToString(r))
	}
}

func TestLocalClipboardWatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	// clear clipboard
	clipboard.Local.Write(types.MIMEPlainText, utils.StringToBytes(""))
	_, lastRead := clipboard.Local.Read()

	dataCh := clipboard.Local.Watch(ctx, types.MIMEPlainText)

	w := utils.StringToBytes("changkun.de/x/midgard")
	go func(ctx context.Context) {
		t := time.NewTicker(time.Millisecond * 500)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				clipboard.Local.Write(types.MIMEPlainText, w)
			}
		}
	}(ctx)
	for {
		select {
		case <-ctx.Done():
			if utils.BytesToString(lastRead) == "" {
				t.Fatalf("clipboard watch never receives a notification")
			}
			return
		case data, ok := <-dataCh:
			if !ok {
				if utils.BytesToString(lastRead) == "" {
					t.Fatalf("clipboard watch never receives a notification")
				}
				return
			}
			if !bytes.Equal(data, w) {
				t.Fatalf("received data from watch mismatch, want: %v, got %v", utils.BytesToString(w), utils.BytesToString(data))
			}
			lastRead = data
		}
	}
}

// TestLocalClipboardUnsupportedMIME pins the contract of the v0.9.0 port:
// a MIME type the OS clipboard does not carry must report a failed write
// rather than claim success, and must not hand back a watch channel.
func TestLocalClipboardUnsupportedMIME(t *testing.T) {
	if clipboard.Local.Write("application/octet-stream", []byte("data")) {
		t.Fatal("write of an unsupported MIME type reported success")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if ch := clipboard.Local.Watch(ctx, "application/octet-stream"); ch != nil {
		t.Fatal("watch of an unsupported MIME type returned a channel")
	}
}

// A repeated write of the same bytes is reported as a success without
// touching the OS clipboard, so that a sync loop does not fight itself.
func TestLocalClipboardWriteIsIdempotent(t *testing.T) {
	data := utils.StringToBytes("changkun.de/x/midgard/idempotent")
	if !clipboard.Local.Write(types.MIMEPlainText, data) {
		t.Fatal("first write failed")
	}
	if !clipboard.Local.Write(types.MIMEPlainText, data) {
		t.Fatal("second write of identical data failed")
	}

	tp, got := clipboard.Local.Read()
	if tp != types.MIMEPlainText || !bytes.Equal(got, data) {
		t.Fatalf("read back (%v, %q), want (%v, %q)",
			tp, got, types.MIMEPlainText, data)
	}
}
