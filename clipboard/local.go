// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package clipboard

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"sync"

	"golang.design/x/midgard/clipboard/internal/cb"
	"golang.design/x/midgard/types"
)

var (
	// ErrEmpty indicates empty clipboard error
	ErrEmpty = errors.New("empty")
	// ErrAccessDenied indicates that access clipboard is denied by the system
	ErrAccessDenied = errors.New("access denied")
)

// lock locks clipboard operation
var lock = sync.Mutex{}

// ReadString reads clipboard as plain text string.
func ReadString() (string, error) {
	lock.Lock()
	defer lock.Unlock()

	buf := cb.Read(types.ClipboardDataTypePlainText)
	if buf == nil {
		return "", ErrEmpty
	}
	return string(buf), nil
}

// ReadImage reads clipboard as an image.Image
func ReadImage() (image.Image, error) {
	lock.Lock()
	defer lock.Unlock()

	buf := cb.Read(types.ClipboardDataTypeImagePNG)
	if buf == nil {
		return nil, ErrEmpty
	}

	return png.Decode(bytes.NewBuffer(buf))
}

// WriteString writes a given string to the clipboard
func WriteString(s string) error {
	lock.Lock()
	defer lock.Unlock()

	ok := cb.Write([]byte(s), types.ClipboardDataTypePlainText)
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

// WriteImage writes a given image to the clipboard
func WriteImage(img image.Image) error {
	lock.Lock()
	defer lock.Unlock()

	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return err
	}

	ok := cb.Write(buf.Bytes(), types.ClipboardDataTypeImagePNG)
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

// Read reads and returns byte-based clipboard data.
func Read() []byte {
	lock.Lock()
	defer lock.Unlock()

	buf := cb.Read(types.ClipboardDataTypePlainText)
	if buf == nil {
		// if we still have nothing, then just ignore it.
		buf = cb.Read(types.ClipboardDataTypeImagePNG)
	}
	return buf
}

// Write writes the given buffer to the clipboard.
func Write(buf []byte) {
	lock.Lock()
	defer lock.Unlock()

	ok := cb.Write(buf, types.ClipboardDataTypePlainText)
	if !ok {
		// if we still have nothing, then just ignore it.
		_ = cb.Write(buf, types.ClipboardDataTypeImagePNG)
	}
}

// Watch watches clipboard changes and closes the dataCh channel if
// the the watch context is canceled.
func Watch(ctx context.Context, dt types.ClipboardDataType, dataCh chan []byte) {
	go cb.Watch(ctx, dt, dataCh)
}
