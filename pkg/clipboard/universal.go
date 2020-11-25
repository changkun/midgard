// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package clipboard

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"sync"

	"golang.design/x/midgard/pkg/types"
)

var (
	// ErrTypeMismatch indicates an error that the existing data inside
	// the universal clipboard is inconsistent with the requested type.
	ErrTypeMismatch = errors.New("type mismatch")
)

type universalClipboard struct {
	typ types.ClipboardDataType
	buf []byte
	m   sync.Mutex
}

func (uc *universalClipboard) Read() (types.ClipboardDataType, []byte) {
	uc.m.Lock()
	defer uc.m.Unlock()
	buf := make([]byte, len(uc.buf))
	copy(buf, uc.buf)
	return uc.typ, buf
}

func (uc *universalClipboard) ReadAsImgage() (image.Image, error) {
	uc.m.Lock()
	defer uc.m.Unlock()

	if uc.typ != types.ClipboardDataTypeImagePNG {
		return nil, ErrTypeMismatch
	}

	return png.Decode(bytes.NewBuffer(uc.buf))
}

func (uc *universalClipboard) Get(t types.ClipboardDataType) []byte {
	uc.m.Lock()
	defer uc.m.Unlock()
	if t != uc.typ {
		return nil
	}

	buf := make([]byte, len(uc.buf))
	copy(buf, uc.buf)
	return buf
}

func (uc *universalClipboard) Put(t types.ClipboardDataType, buf []byte) {
	uc.m.Lock()
	defer uc.m.Unlock()
	uc.typ = t
	uc.buf = buf
}

// Universal is the Midgard's universal clipboard.
//
// It holds a global shared storage that can be edited/fetched at anytime.
var Universal = universalClipboard{
	typ: types.ClipboardDataTypePlainText,
	buf: []byte{},
	m:   sync.Mutex{},
}
