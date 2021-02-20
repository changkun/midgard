// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard_test

import (
	"bytes"
	"context"
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
	if !reflect.DeepEqual(r, data) || tp != types.MIMEImagePNG {
		t.Fatalf("inconsistent read of a write, got: %s", utils.BytesToString(r))
	}
	t.Log(tp)
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
			if bytes.Compare(data, w) != 0 {
				t.Fatalf("received data from watch mismatch, want: %v, got %v", utils.BytesToString(w), utils.BytesToString(data))
			}
			lastRead = data
		}
	}
}
