// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard

import (
	"bytes"
	"context"
	"errors"
	"sync"

	"changkun.de/x/midgard/internal/clipboard/internal/cb"
	"changkun.de/x/midgard/internal/types"
)

var (
	// ErrEmpty indicates empty clipboard error
	ErrEmpty = errors.New("empty")
	// ErrAccessDenied indicates that access clipboard is denied by the system
	ErrAccessDenied = errors.New("access denied")
)

// lock locks clipboard operation
var (
	lock     = sync.Mutex{}
	localbuf = []byte{} // hold a local copy
)

// Read reads and returns byte-based clipboard data.
func Read() []byte {
	lock.Lock()
	defer lock.Unlock()

	buf := cb.Read(types.ClipboardDataTypePlainText)
	if buf == nil {
		// if we still have nothing, then just ignore it.
		buf = cb.Read(types.ClipboardDataTypeImagePNG)
	}
	localbuf = buf
	return buf
}

// Write writes the given buffer to the clipboard.
func Write(buf []byte) {
	lock.Lock()
	defer lock.Unlock()

	// if the local copy is the same with the write, do not bother.
	if bytes.Compare(localbuf, buf) == 0 {
		return
	}
	localbuf = buf

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

// HandleHotkey registers an application global hotkey to the system,
// and returns a channel that will signal if the hotkey is triggered.
//
// No customization for the hotkey, the hotkey is always:
// Linux: Ctrl+Mod4+s
// macOS: Ctrl+Option+s
func HandleHotkey(ctx context.Context, fn func()) {
	go cb.HandleHotkey(ctx, fn)
}
