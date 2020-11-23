// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package clipboard_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"golang.design/x/midgard/clipboard"
	"golang.design/x/midgard/types"
)

func TestLocalClipboardImage(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/gold.png")
	if err != nil {
		t.Fatalf("failed to read gold file, err: %v", err)
	}
	clipboard.Write(data)

	r := clipboard.Read()
	if !reflect.DeepEqual(r, data) {
		t.Fatalf("inconsistent read of a write, got: %s", string(r))
	}
}

func TestLocalClipboardText(t *testing.T) {
	data := []byte("golang.design/x/midgard")
	clipboard.Write(data)

	r := clipboard.Read()
	if !reflect.DeepEqual(r, data) {
		t.Fatalf("inconsistent read of a write, got: %s", string(r))
	}
}

func TestLocalClipboardWatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	// clear clipboard
	clipboard.Write([]byte(""))
	lastRead := clipboard.Read()

	dataCh := make(chan []byte, 1)
	clipboard.Watch(ctx, types.ClipboardDataTypePlainText, dataCh)

	w := []byte("golang.design/x/midgard")
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
			if string(lastRead) == "" {
				t.Fatalf("clipboard watch never receives a notification")
			}
			return
		case data, ok := <-dataCh:
			if !ok {
				if string(lastRead) == "" {
					t.Fatalf("clipboard watch never receives a notification")
				}
				return
			}
			if bytes.Compare(data, w) != 0 {
				t.Fatalf("received data from watch mismatch, want: %v, got %v", string(w), string(data))
			}
			lastRead = data
		}
	}
}
