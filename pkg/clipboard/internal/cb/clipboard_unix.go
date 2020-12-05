// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// +build freebsd linux netbsd openbsd solaris dragonfly

package cb

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
int wait_hotkey() {
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
			return 0;
		}
	}
}
*/
import "C"
import (
	"bytes"
	"context"
	"os/exec"
	"time"

	"changkun.de/x/midgard/pkg/types"
)

var (
	xclip        = "xclip"
	pasteCmdArgs = []string{xclip, "-out", "-selection", "clipboard"}
	copyCmdArgs  = []string{xclip, "-in", "-selection", "clipboard"}
)

func init() {
	if _, err := exec.LookPath(xclip); err == nil {
		return
	}
	panic("please intall xclip on your system: sudo apt install xclip")
}

// Read reads the clipboard data of a given resource type.
// It returns a buf that containing the clipboard data, and ok indicates
// whether the read is success or fail.
func Read(t types.ClipboardDataType) (buf []byte) {
	cmds := make([]string, len(pasteCmdArgs))
	copy(cmds, pasteCmdArgs)
	if t == types.ClipboardDataTypeImagePNG {
		cmds = append(cmds, "-t", "image/png")
	}
	pasteCmd := exec.Command(cmds[0], cmds[1:]...)
	out, err := pasteCmd.CombinedOutput()
	if err != nil {
		return nil
	}
	return out
}

// Write writes the given buf as typ to system clipboard.
// It returns true if the write is success.
func Write(buf []byte, t types.ClipboardDataType) (ret bool) {
	copyCmd := exec.Command(copyCmdArgs[0], copyCmdArgs[1:]...)
	in, err := copyCmd.StdinPipe()
	if err != nil {
		return false
	}

	if err := copyCmd.Start(); err != nil {
		return false
	}
	if _, err := in.Write(buf); err != nil {
		return false
	}
	if err := in.Close(); err != nil {
		return false
	}
	if err := copyCmd.Wait(); err != nil {
		return false
	}
	return true
}

// Watch watches the changes of system clipboard, and sends the data of
// clipboard to the given dataCh.
func Watch(ctx context.Context, dt types.ClipboardDataType, dataCh chan []byte) {
	// FIXME: this is not the ideal approach. On linux, we can interact
	// with X11 ICCCM to listen to the selection notification event,
	// then trigger the watch as needed to avoid frequent Read().
	t := time.NewTicker(time.Second)
	last := Read(dt)
	for {
		select {
		case <-ctx.Done():
			close(dataCh)
			return
		case <-t.C:
			b := Read(dt)
			if b == nil {
				continue
			}
			if bytes.Compare(last, b) != 0 {
				dataCh <- b
				last = b
			}
		}
	}
}

// HandleHotkey registers an application global hotkey to the system,
// and returns a channel that will signal if the hotkey is triggered.
//
// No customization for the hotkey, the hotkey is always: Ctrl+Mod4+s
func HandleHotkey(ctx context.Context, fn func()) {
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
