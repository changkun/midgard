// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// +build linux

package platform

/*
#cgo LDFLAGS: -lX11 -lXmu
#include <stdlib.h>
#include <stdio.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xmu/Atoms.h>

static Display* d;
static Window w;

static Atom clipboardSel;
static Atom stringAtom;
static Atom imageAtom;
static Atom cbAtom;

int clipboard_init() {
	XInitThreads();

	d = XOpenDisplay(0);
	w = XCreateSimpleWindow(d, DefaultRootWindow(d), 0, 0, 1, 1, 0, 0, 0);

	clipboardSel = XInternAtom(d, "CLIPBOARD", True);
	stringAtom   = XInternAtom(d, "UTF8_STRING", True);
	imageAtom    = XInternAtom(d, "image/png", True);
	cbAtom       = XInternAtom(d, "GOLANG_DESIGN_DATA", 0);
	return 0;
}

int clipboard_write(char* target_typ, unsigned char *_in, size_t _n) {
	Atom target = XInternAtom(d, target_typ, True);
	if (target == None) {
		return -1;
	}

	XEvent event;
	Window owner;
	XSetSelectionOwner(d, clipboardSel, w, 0);
	if (XGetSelectionOwner(d, clipboardSel) != w) {
		// printf("no cannot own\n");
		// fflush(stdout);
		return -1;
	}

	while (1) {
		printf("enter selection event loop, wait for request...\n");
		fflush(stdout);

		XNextEvent(d, &event);
		switch (event.type) {
		case SelectionRequest:
			if (event.xselectionrequest.selection != clipboardSel) {
				break;
			}
			XSelectionRequestEvent * xsr = &event.xselectionrequest;
			XSelectionEvent ev = {0};
			int R = 0;
			ev.type = SelectionNotify;
			ev.display = xsr->display;
			ev.requestor = xsr->requestor;
			ev.selection = xsr->selection;
			ev.time = xsr->time;
			ev.target = xsr->target;
			ev.property = xsr->property;
			// printf("event atom: %s\n", XGetAtomName(d, ev.target));
			// fflush(stdout);

			if (ev.target == target) {
				R = XChangeProperty(ev.display, ev.requestor,
					ev.property, XA_ATOM, 32, PropModeReplace, (unsigned char*)&stringAtom, 1);
			} else {
				return 0;
			}
			if ((R & 2) == 0) {
				XSendEvent(d, ev.requestor, 0, 0, (XEvent *)&ev);
			}
			break;

		case SelectionClear: // stop surve the write content if it is cleared.
			printf("selection is cleared\n");
			fflush(stdout);
			return 0;
		}
	}
}

unsigned long clipboard_read(char* target_typ, unsigned char **out) {
	Atom target = XInternAtom(d, target_typ, True);
	if (target == None) {
		return 0;
	}

	XConvertSelection(d, clipboardSel, target, cbAtom, w, CurrentTime);
	XSync(d, 0);
	XEvent event;
	XNextEvent(d, &event);

	unsigned char *in;
	long n;
	Atom actual;
	int format;
	unsigned long size = 0;
	size_t itemsize = 0;

	if (event.type == SelectionNotify &&
		event.xselection.selection == clipboardSel &&
		event.xselection.property)
	{
		if (XGetWindowProperty(event.xselection.display,
			event.xselection.requestor, event.xselection.property,
			0L, (~0L), 0, AnyPropertyType,
			&actual, &format, &size, &n, &in) == Success) {
			// printf("actual: %s, target: %s\n", XGetAtomName(d, actual), XGetAtomName(d, target));
			// fflush(stdout);
			if (actual == target) {
				if (format == 8) {
					itemsize = sizeof(char);
				} else if (format == 16) {
					itemsize = sizeof(short);
				} else if (format == 32) {
					itemsize = sizeof(long);
				}
				void *recv = (unsigned char *)malloc(size * itemsize);
				if (recv != NULL) {
					memcpy(recv, in, size * itemsize);
					*out = recv;
				}
			}
			XFree(in);
		};
		XDeleteProperty(event.xselection.display,
			event.xselection.requestor, event.xselection.property);
	}
	return size * itemsize;
}
*/
import "C"
import (
	"bytes"
	"context"
	"time"
	"unsafe"

	"changkun.de/x/midgard/internal/types"
)

func init() {
	if ret := C.clipboard_init(); ret == 0 {
		return
	}
	panic("cannot initialize clipboard!")
}

// Read reads the clipboard data of a given resource type.
// It returns a buf that containing the clipboard data, and ok indicates
// whether the read is success or fail.
func Read(t types.MIME) (buf []byte) {
	ctyp := C.CString(string(t))
	defer C.free(unsafe.Pointer(ctyp))

	var data *C.uchar
	n := C.clipboard_read(ctyp, &data)
	if data == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(data))
	if n == 0 {
		return nil
	}

	return C.GoBytes(unsafe.Pointer(data), C.int(n))
}

// Write writes the given buf as typ to system clipboard.
// It returns true if the write is success.
func Write(buf []byte, t types.MIME) (ret bool) {
	var s string
	switch t {
	case types.MIMEPlainText:
		s = "UTF8_STRING"
	case types.MIMEImagePNG:
		s = "image/png"
	}

	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))

	ok := C.clipboard_write(cs, (*C.uchar)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)))
	if ok != C.int(0) {
		return false
	}
	return true
}

// Watch watches the changes of system clipboard, and sends the data of
// clipboard to the given dataCh.
func Watch(ctx context.Context, dt types.MIME, dataCh chan []byte) {
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
