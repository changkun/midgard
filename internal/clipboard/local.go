// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard

import (
	"bytes"
	"context"
	"sync"

	"changkun.de/x/midgard/internal/clipboard/platform"
	"changkun.de/x/midgard/internal/types"
)

// LocalClipboard is an extension to the Clipboard interface
// for local purpose
type LocalClipboard interface {
	Clipboard
	// Watch watches a given type of data from local clipboard and
	// send the data back through a provided channel.
	Watch(ctx context.Context, dt types.MIME, dataCh chan []byte)
}

// Local is a local clipboard that can interact with the OS clipboard.
var Local LocalClipboard = &local{
	buf: []byte{},
}

type local struct {
	sync.Mutex
	buf []byte
	typ types.MIME
}

// Read reads and returns byte-based clipboard data.
func (lc *local) Read() (t types.MIME, buf []byte) {
	lc.Lock()
	defer lc.Unlock()
	defer func() {
		lc.buf = buf
	}()

	buf = platform.Read(types.MIMEPlainText)
	if buf != nil {
		t = types.MIMEPlainText
		return
	}
	buf = platform.Read(types.MIMEImagePNG)
	t = types.MIMEImagePNG
	return
}

// Write writes the given buffer to the clipboard.
func (lc *local) Write(t types.MIME, buf []byte) bool {
	lc.Lock()
	defer lc.Unlock()

	// if the local copy is the same with the write, do not bother.
	if bytes.Compare(lc.buf, buf) == 0 {
		return true // but we recognize it as a success write
	}
	lc.buf = buf
	lc.typ = t
	return platform.Write(buf, t)
}

// Watch watches clipboard changes and closes the dataCh channel if
// the the watch context is canceled.
func (lc *local) Watch(ctx context.Context, dt types.MIME, dataCh chan []byte) {
	go platform.Watch(ctx, dt, dataCh)
}
