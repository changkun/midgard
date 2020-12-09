// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// +build freebsd linux netbsd openbsd solaris dragonfly

package hotkey

/*
#cgo LDFLAGS: -lX11 -lXmu
#include <stdio.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>

// wait_hotkey blocks until the hotkey is triggered
//
// Mask        | Value | Key
// ------------+-------+------------
// ShiftMask   |     1 | Shift
// LockMask    |     2 | Caps Lock
// ControlMask |     4 | Ctrl
// Mod1Mask    |     8 | Alt
// Mod2Mask    |    16 | Num Lock
// Mod3Mask    |    32 | Scroll Lock
// Mod4Mask    |    64 | Windows
// Mod5Mask    |   128 | ???
//
void wait_hotkey() {
	Display* dpy = XOpenDisplay(0);

	// default keys
	// Control+Mod2+Mod4 + s | m:0x5c + c:44
	unsigned int modifiers = ControlMask | Mod2Mask | Mod4Mask;
	int keycode = XKeysymToKeycode(dpy, XK_s);

	// one can use xbindkeys-config
	XGrabKey(dpy, keycode, modifiers, DefaultRootWindow(dpy),
		False, GrabModeAsync, GrabModeAsync);
	XSelectInput(dpy, DefaultRootWindow(dpy), KeyPressMask );

	XEvent ev;
	while(1) {
		XNextEvent(dpy, &ev);
		switch(ev.type) {
		case KeyPress:
			XUngrabKey(dpy, keycode, modifiers, DefaultRootWindow(dpy));
			XCloseDisplay(dpy);
			return;
		}
	}
}
*/
import "C"
import "context"

// handle registers an application global hotkey to the system,
// and returns a channel that will signal if the hotkey is triggered.
//
// No customization for the hotkey, the hotkey is always: Ctrl+Mod4+s
func handle(ctx context.Context, fn func()) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			C.wait_hotkey()
			fn()
		}
	}

}
