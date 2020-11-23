// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package api

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"sync"

	"golang.design/x/midgard/clipboard"
)

var (
	// ErrTypeMismatch indicates an error that the existing data inside
	// the universal clipboard is inconsistent with the requested type.
	ErrTypeMismatch = errors.New("type mismatch")
)

type universalClipboard struct {
	Type clipboard.DataType
	Data []byte
	m    sync.Mutex
}

func (uc *universalClipboard) read() (clipboard.DataType, []byte) {
	uc.m.Lock()
	defer uc.m.Unlock()
	buf := make([]byte, len(uc.Data))
	copy(buf, uc.Data)
	return uc.Type, buf
}

func (uc *universalClipboard) readAsImgage() (image.Image, error) {
	uc.m.Lock()
	defer uc.m.Unlock()

	if uc.Type != clipboard.DataTypeImagePNG {
		return nil, ErrTypeMismatch
	}

	return png.Decode(bytes.NewBuffer(uc.Data))
}

func (uc *universalClipboard) get(t clipboard.DataType) []byte {
	uc.m.Lock()
	defer uc.m.Unlock()
	if t != uc.Type {
		return nil
	}

	buf := make([]byte, len(uc.Data))
	copy(buf, uc.Data)
	return buf
}

func (uc *universalClipboard) put(t clipboard.DataType, buf []byte) {
	uc.m.Lock()
	defer uc.m.Unlock()
	uc.Type = t
	uc.Data = buf
}

// uc is the Midgard's universal clipboard, it holds a global shared
// storage that can be edited/fetched at anytime.
var uc0 = universalClipboard{
	Type: clipboard.DataTypePlainText,
	Data: []byte{},
	m:    sync.Mutex{},
}
