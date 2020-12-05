// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// +build darwin

package cb

// Interact with NSPasteboard using Objective-C
// https://developer.apple.com/documentation/appkit/nspasteboard?language=objc

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework Carbon
#import <Foundation/Foundation.h>
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h> // for keyboard hotkey

unsigned int clipboard_read_string(void **out);
unsigned int clipboard_read_image(void **out);
int clipboard_write_string(const void *bytes, NSInteger n);
int clipboard_write_image(const void *bytes, NSInteger n);
NSInteger clipboard_change_count();

extern void go_hotkey_callback(void* handler);
static OSStatus _hotkey_handler(EventHandlerCallRef nextHandler, EventRef theEvent, void *userData);
int register_hotkey(void* go_hotkey_handler);
void run_shared_application();
*/
import "C"
import (
	"context"
	"log"
	"sync"
	"time"
	"unsafe"

	"changkun.de/x/midgard/pkg/mainthread"
	"changkun.de/x/midgard/pkg/types"
)

var (
	lock sync.Mutex
	once sync.Once
)

// Read reads the clipboard data of a given resource type.
// It returns a buf that containing the clipboard data, and ok indicates
// whether the read is success or fail.
func Read(t types.ClipboardDataType) (buf []byte) {
	// Concurrent read/write clipboard on macOS can cause crashes
	// One must serialize the operation to the clipboard.
	lock.Lock()
	defer lock.Unlock()

	var (
		data unsafe.Pointer
		n    C.uint
	)
	switch t {
	case types.ClipboardDataTypePlainText:
		n = C.clipboard_read_string(&data)
	case types.ClipboardDataTypeImagePNG:
		n = C.clipboard_read_image(&data)
	}
	if data == nil || n == 0 {
		return nil
	}
	defer C.free(unsafe.Pointer(data)) // malloced from C
	return C.GoBytes(data, C.int(n))
}

// Write writes the given buf as typ to system clipboard.
// It returns true if the write is success.
func Write(buf []byte, t types.ClipboardDataType) (ret bool) {
	// Concurrent read/write clipboard on macOS can cause crashes
	// One must serialize the operation to the clipboard.
	lock.Lock()
	defer lock.Unlock()

	if buf == nil {
		return true
	}

	var ok C.int
	switch t {
	case types.ClipboardDataTypePlainText:
		ok = C.clipboard_write_string(unsafe.Pointer(&buf[0]),
			C.NSInteger(len(buf)))
	case types.ClipboardDataTypeImagePNG:
		ok = C.clipboard_write_image(unsafe.Pointer(&buf[0]),
			C.NSInteger(len(buf)))
	}

	if ok != 0 {
		return false
	}
	return true
}

// Watch watches the changes of system clipboard, and sends the data of
// clipboard to the given dataCh.
//
// Unfortunately, on macOS, NSPasteboard does not offer a way to listen
// clipboard changes. This is a workaround method to fetch the property
// of pasteboard change count. If the change count is different than
// what we have before, then meaning the clipboard is change, and we can
// read the data, see:
// https://developer.apple.com/library/archive/samplecode/ClipboardViewer/Introduction/Intro.html#//apple_ref/doc/uid/DTS40008825-Intro-DontLinkElementID_2
//
// FIXME: Alternatively, we could watch keyboard hotkeys, for instance,
// a double cmd+c triggers the watch? Needs invesgitation.
func Watch(ctx context.Context, dt types.ClipboardDataType, dataCh chan []byte) {
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
				b := Read(dt)
				if b == nil {
					continue
				}
				dataCh <- b
				lastCount = this
			}
		}
	}
}

// This hkCallback tries to avoid a runtime panic error when directly
// pass it to Cgo:
//   panic: runtime error: cgo argument has Go pointer to Go pointer
var (
	hkCallback func()
	hkMu       sync.Mutex
)

// HandleHotkey registers an application global hotkey to the system,
// and returns a channel that will signal if the hotkey is triggered.
//
// No customization for the hotkey, the hotkey is always: Ctrl+Option+s
func HandleHotkey(ctx context.Context, fn func()) {
	hkMu.Lock()
	hkCallback = fn
	hkMu.Unlock()

	// make sure the registration is on the mainthread, don't ask why.
	mainthread.Call(func() {
		arg := unsafe.Pointer(&gocallback{func() {
			hkMu.Lock()
			f := hkCallback // must use a global function variable.
			hkMu.Unlock()

			f()
		}})
		ret := C.register_hotkey(arg)
		if ret == C.int(-1) {
			log.Println("register global system hotkey failed.")
		}

		C.run_shared_application()
	})
}

type gocallback struct{ f func() }

func (c *gocallback) call() { c.f() }

//export go_hotkey_callback
func go_hotkey_callback(c unsafe.Pointer) {
	(*gocallback)(c).call()
}
