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
)

var (
	// ErrEmpty indicates empty clipboard error
	ErrEmpty = errors.New("empty")
	// ErrAccessDenied indicates that access clipboard is denied by the system
	ErrAccessDenied = errors.New("access denied")
)

// DataType indicates clipboard data type
type DataType int

const (
	// DataTypePlainText indicates plain text data type
	DataTypePlainText DataType = iota
	// DataTypeImagePNG indicates image/png data type
	DataTypeImagePNG
)

// Data is a clipboard data
type Data struct {
	Type DataType `json:"type"`
	Data string   `json:"data"` // base64 encode if type is an image data
}

// lock locks clipboard operation
var lock = sync.Mutex{}

// ReadString reads clipboard as plain text string.
func ReadString() (string, error) {
	lock.Lock()
	defer lock.Unlock()

	buf := read(DataTypePlainText)
	if buf == nil {
		return "", ErrEmpty
	}
	return string(buf), nil
}

// ReadImage reads clipboard as an image.Image
func ReadImage() (image.Image, error) {
	lock.Lock()
	defer lock.Unlock()

	buf := read(DataTypeImagePNG)
	if buf == nil {
		return nil, ErrEmpty
	}

	return png.Decode(bytes.NewBuffer(buf))
}

// WriteString writes a given string to the clipboard
func WriteString(s string) error {
	lock.Lock()
	defer lock.Unlock()

	ok := write([]byte(s), DataTypePlainText)
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

	ok := write(buf.Bytes(), DataTypeImagePNG)
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

// Read reads and returns byte-based clipboard data.
func Read() []byte {
	lock.Lock()
	defer lock.Unlock()

	buf := read(DataTypePlainText)
	if buf == nil {
		// if we still have nothing, then just ignore it.
		buf = read(DataTypeImagePNG)
	}
	return buf
}

// Write writes the given buffer to the clipboard.
func Write(buf []byte) {
	lock.Lock()
	defer lock.Unlock()

	ok := write(buf, DataTypePlainText)
	if !ok {
		// if we still have nothing, then just ignore it.
		_ = write(buf, DataTypeImagePNG)
	}
}

// Watch watches clipboard changes and closes the dataCh channel if
// the the watch context is canceled.
func Watch(ctx context.Context, dt DataType, dataCh chan []byte) {
	go watch(ctx, dt, dataCh)
}
