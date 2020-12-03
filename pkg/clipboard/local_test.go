// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"changkun.de/x/midgard/pkg/clipboard"
	"changkun.de/x/midgard/pkg/types"
	"changkun.de/x/midgard/pkg/utils"
)

func TestLocalClipboardImage(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/gold.png")
	if err != nil {
		t.Fatalf("failed to read gold file, err: %v", err)
	}
	clipboard.Write(data)

	r := clipboard.Read()
	if !reflect.DeepEqual(r, data) {
		t.Fatalf("inconsistent read of a write, got: %s", utils.BytesToString(r))
	}
}

func TestLocalClipboardText(t *testing.T) {
	data := utils.StringToBytes("changkun.de/x/midgard")
	clipboard.Write(data)

	r := clipboard.Read()
	if !reflect.DeepEqual(r, data) {
		t.Fatalf("inconsistent read of a write, got: %s", utils.BytesToString(r))
	}
}

func TestLocalClipboardWatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	// clear clipboard
	clipboard.Write(utils.StringToBytes(""))
	lastRead := clipboard.Read()

	dataCh := make(chan []byte, 1)
	clipboard.Watch(ctx, types.ClipboardDataTypePlainText, dataCh)

	w := utils.StringToBytes("changkun.de/x/midgard")
	go func(ctx context.Context) {
		t := time.NewTicker(time.Millisecond * 500)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				clipboard.Write(w)
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
