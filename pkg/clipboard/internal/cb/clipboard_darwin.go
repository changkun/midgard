// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

// +build darwin

package cb

// Interact with NSPasteboard using Objective-C
// https://developer.apple.com/documentation/appkit/nspasteboard?language=objc

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#import <Foundation/Foundation.h>
#import <Cocoa/Cocoa.h>
unsigned int clipboard_read_string(void **out) {
	NSPasteboard * pasteboard = [NSPasteboard generalPasteboard];
	NSData *data = [pasteboard dataForType:NSPasteboardTypeString];
	if (data == nil) {
		return 0;
	}
	NSUInteger siz = [data length];
	*out = malloc(siz);
	[data getBytes: *out length: siz];
	return siz;
}
unsigned int clipboard_read_image(void **out) {
	NSPasteboard * pasteboard = [NSPasteboard generalPasteboard];
	NSData *data = [pasteboard dataForType:NSPasteboardTypePNG];
	if (data == nil) {
		return 0;
	}
	NSUInteger siz = [data length];
	*out = malloc(siz);
	[data getBytes: *out length: siz];
	return siz;
}
int clipboard_write_string(const void *bytes, NSInteger n) {
	NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
	NSData *data = [NSData dataWithBytes: bytes length: n];
	[pasteboard clearContents];
	BOOL ok = [pasteboard setData: data forType:NSPasteboardTypeString];
	if (!ok) {
		return -1;
	}
	return 0;
}
int clipboard_write_image(const void *bytes, NSInteger n) {
	NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
	NSData *data = [NSData dataWithBytes: bytes length: n];
	[pasteboard clearContents];
	BOOL ok = [pasteboard setData: data forType:NSPasteboardTypePNG];
	if (!ok) {
		return -1;
	}
	return 0;
}

NSInteger clipboard_change_count() {
	NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
	return pasteboard.changeCount;
}

*/
import "C"
import (
	"context"
	"time"
	"unsafe"
)

// Read reads the clipboard data of a given resource type.
// It returns a buf that containing the clipboard data, and ok indicates
// whether the read is success or fail.
func Read(t DataType) (buf []byte) {
	var (
		data unsafe.Pointer
		n    C.uint
	)
	switch t {
	case DataTypePlainText:
		n = C.clipboard_read_string(&data)
	case DataTypeImagePNG:
		n = C.clipboard_read_image(&data)
	}
	if data == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(data))
	if n == 0 {
		return nil
	}
	return C.GoBytes(data, C.int(n))
}

// Write writes the given buf as typ to system clipboard.
// It returns true if the write is success.
func Write(buf []byte, t DataType) (ret bool) {
	var ok C.int

	switch t {
	case DataTypePlainText:
		ok = C.clipboard_write_string(unsafe.Pointer(&buf[0]),
			C.NSInteger(len(buf)))
	case DataTypeImagePNG:
		ok = C.clipboard_write_image(unsafe.Pointer(&buf[0]),
			C.NSInteger(len(buf)))
	}

	if ok != 0 {
		return false
	}
	return true
}

// watch watches the changes of system clipboard, and sends the data of
// clipboard to the given dataCh.
//
// Unfortunately, on macOS, NSPasteboard does not offer a way to listen
// clipboard changes. This is a workaround method to fetch the property
// of pasteboard change count. If the change count is different than
// what we have before, then meaning the clipboard is change, and we can
// read the data, see:
// https://developer.apple.com/library/archive/samplecode/ClipboardViewer/Introduction/Intro.html#//apple_ref/doc/uid/DTS40008825-Intro-DontLinkElementID_2
//
// TODO: Alternatively, we could watch keyboard hotkeys, for instance,
// a double cmd+c triggers the watch? Needs invesgitation.
func Watch(ctx context.Context, dt DataType, dataCh chan []byte) {
	// we try to watch the clipboard every second, this should be enough
	// for the watch purpose. If the user is too fast, meaning be able
	// to paste the content within a second, then it is very unfortunate.
	t := time.NewTicker(time.Second)
	lastCount := C.long(C.clipboard_change_count())
	for {
		select {
		case <-ctx.Done():
			close(dataCh)
			return
		case <-t.C:
			this := C.long(C.clipboard_change_count())
			if lastCount != this {
				bytes := read(dt)
				if bytes == nil {
					continue
				}
				dataCh <- bytes
				lastCount = this
			}
		}
	}
}
