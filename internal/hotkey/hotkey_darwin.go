// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// +build darwin

package hotkey

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Carbon
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>

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
	"unsafe"

	"golang.design/x/mainthread"
)

// This hkCallback tries to avoid a runtime panic error when directly
// pass it to Cgo:
//   panic: runtime error: cgo argument has Go pointer to Go pointer
var (
	hkCallback func()
	hkMu       sync.Mutex
)

// handle registers an application global hotkey to the system,
// and returns a channel that will signal if the hotkey is triggered.
//
// No customization for the hotkey, the hotkey is always: Ctrl+Option+s
func handle(ctx context.Context, fn func()) {
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
		log.Println("hotkey is registered.")
		C.run_shared_application()
	})
}

type gocallback struct{ f func() }

func (c *gocallback) call() { c.f() }

//export go_hotkey_callback
func go_hotkey_callback(c unsafe.Pointer) {
	(*gocallback)(c).call()
}
