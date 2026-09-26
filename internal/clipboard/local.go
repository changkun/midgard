// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard

import (
	"bytes"
	"context"
	"log/slog"
	"sync"

	"changkun.de/x/midgard/internal/types"
	"golang.design/x/clipboard"
)

// Clipboard is an interface that defines the operations of a clipboard
type Clipboard interface {
	// Read reads the clipboard and returns the MIME type and
	// the raw bytes data in the clipboard
	Read() (types.MIME, []byte)
	// Write write the given data as the given MIME type and
	// returns true if success or false if failed.
	Write(types.MIME, []byte) bool
}

// LocalClipboard is an extension to the Clipboard interface
// for local purpose
type LocalClipboard interface {
	Clipboard
	// Watch watches a given type of data from local clipboard and
	// send the data back through a provided channel.
	Watch(ctx context.Context, dt types.MIME) <-chan []byte
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

// ready reports whether the OS clipboard is usable, initializing it on the
// first call. Initialization is lazy because the midgard server never reads
// the local clipboard, and a headless host has none to initialize.
var ready = sync.OnceValue(func() bool {
	if err := clipboard.Init(); err != nil {
		slog.Warn("the local clipboard is unavailable", "err", err)
		return false
	}
	return true
})

// pngSignature is how every PNG file begins.
var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// format maps a midgard MIME type to a clipboard format. ok is false for a
// MIME type the local clipboard does not carry.
func format(t types.MIME) (f clipboard.Format, ok bool) {
	switch t {
	case types.MIMEPlainText:
		return clipboard.FmtText, true
	case types.MIMEImagePNG:
		return clipboard.FmtImage, true
	}
	return 0, false
}

// Read reads and returns byte-based clipboard data.
func (lc *local) Read() (t types.MIME, buf []byte) {
	if !ready() {
		return types.MIMEPlainText, nil
	}

	lc.Lock()
	defer lc.Unlock()
	defer func() {
		lc.buf = buf
	}()

	ctx := context.Background()
	buf, err := clipboard.Read(ctx, clipboard.FmtText)
	if err != nil {
		slog.Error("cannot read text from the local clipboard", "err", err)
	}
	if buf != nil {
		return types.MIMEPlainText, buf
	}

	buf, err = clipboard.Read(ctx, clipboard.FmtImage)
	if err != nil {
		slog.Error("cannot read an image from the local clipboard", "err", err)
	}
	return types.MIMEImagePNG, buf
}

// Write writes the given buffer to the clipboard.
func (lc *local) Write(t types.MIME, buf []byte) bool {
	f, ok := format(t)
	if !ok || !ready() {
		return false
	}

	lc.Lock()
	defer lc.Unlock()

	// if the local copy is the same with the write, do not bother.
	if bytes.Equal(lc.buf, buf) {
		return true // but we recognize it as a success write
	}

	if _, err := clipboard.Write(context.Background(), f, buf); err != nil {
		slog.Error("cannot write to the local clipboard", "mime", t, "err", err)
		return false
	}
	lc.buf = buf
	lc.typ = t
	return true
}

// Watch watches clipboard changes and closes the dataCh channel if
// the the watch context is canceled.
func (lc *local) Watch(ctx context.Context, dt types.MIME) <-chan []byte {
	f, ok := format(dt)
	if !ok || !ready() {
		return nil
	}

	out := make(chan []byte)
	go forward(ctx, clipboard.Watch(ctx, f), out)
	return out
}

// forward passes each change on as its bytes, and closes out when src is
// done. clipboard.Watch reports the format alongside the bytes so that one
// call can observe several formats; midgard watches one format per channel.
//
// A change the copying application marked as sensitive — a password from a
// password manager — is dropped here, so it never leaves the machine. Before,
// every password copied on any device was synced to the server and on to
// every other device.
//
// An image that is not a PNG is dropped too. The clipboard asks the copying
// application for image/png, and some answer every request with what they
// hold: xclip, given text, gives the text for image/png as well. It was
// synced as an image, and put an image nobody copied on the other devices.
func forward(ctx context.Context, src <-chan clipboard.Data, out chan<- []byte) {
	defer close(out)
	for d := range src {
		if d.Sensitive {
			slog.Info("not syncing a copy marked as sensitive")
			continue
		}
		if d.Format == clipboard.FmtImage && !bytes.HasPrefix(d.Bytes, pngSignature) {
			continue
		}
		select {
		case out <- d.Bytes:
		case <-ctx.Done():
			return
		}
	}
}
