// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Command engine is midgard's sync engine as a C library, for the Mac app
// (specs/redesign.md §3). The same engine as mg daemon's, internal/device,
// so there is one: the app owns the pasteboard, the hotkey and the window,
// and calls this for everything else.
//
//	go build -buildmode=c-archive -o libmidgard.a ./apple/engine
//
// builds libmidgard.a and libmidgard.h. A function that can fail returns
// NULL, or why it failed; a string or bytes it returns are the caller's, to
// give back with MidgardFree. Anything structured is JSON.
package main

/*
#include <stdint.h>
#include <stdlib.h>

// MidgardChanged is called, on a thread of the engine's, with a copy from
// another device that belongs on the clipboard. The bytes are the engine's,
// and only for the length of the call.
typedef void (*MidgardChanged)(const char *mime, const void *data, int64_t size);

// MidgardOpen is called with the link to approve a sign-in at.
typedef void (*MidgardOpen)(const char *link);

static inline void midgardCallChanged(MidgardChanged f, const char *mime, const void *data, int64_t size) {
	if (f) f(mime, data, size);
}
static inline void midgardCallOpen(MidgardOpen f, const char *link) {
	if (f) f(link);
}
*/
import "C"

import (
	"errors"
	"unsafe"

	"changkun.de/x/midgard/internal/device"
)

func main() {}

// cerr is err for C: NULL, or why.
func cerr(err error) *C.char {
	if err == nil {
		return nil
	}
	return C.CString(err.Error())
}

// MidgardStart starts the sync. It fails with *code 1 until MidgardSetup
// has set the server, with 2 while mg daemon, or another app, syncs this
// device, and with 3 otherwise; 0 is success.
//
//export MidgardStart
func MidgardStart(changed C.MidgardChanged, code *C.int) *C.char {
	err := start(func(mime string, data []byte) {
		m := C.CString(mime)
		defer C.free(unsafe.Pointer(m))
		p := C.CBytes(data)
		defer C.free(p)
		C.midgardCallChanged(changed, m, p, C.int64_t(len(data)))
	})
	switch {
	case err == nil:
		*code = 0
	case errors.Is(err, errNoServer):
		*code = 1
	case errors.Is(err, device.ErrRunning):
		*code = 2
	default:
		*code = 3
	}
	return cerr(err)
}

// MidgardStop stops the sync.
//
//export MidgardStop
func MidgardStop() { shutdown() }

// MidgardSetup sets the server, such as example.com or http://mg.local:8456.
//
//export MidgardSetup
func MidgardSetup(domain *C.char) *C.char { return cerr(setup(C.GoString(domain))) }

// MidgardCopy records a copy made on the Mac, to send to the other devices.
// The app leaves out what a password manager marks as secret.
//
//export MidgardCopy
func MidgardCopy(mime *C.char, data unsafe.Pointer, size C.int64_t) *C.char {
	return cerr(copyData(C.GoString(mime), C.GoBytes(data, C.int(size))))
}

// MidgardDelete removes copy seq from the history, on every device.
//
//export MidgardDelete
func MidgardDelete(seq C.uint64_t) *C.char { return cerr(deleteCopy(uint64(seq))) }

// MidgardClear removes every copy from the history, on every device.
//
//export MidgardClear
func MidgardClear() *C.char { return cerr(clearHistory()) }

// MidgardHistory is the newest n copies as JSON, newest first:
// [{"seq","time","origin","mime","size","preview","waiting"}], or NULL.
//
//export MidgardHistory
func MidgardHistory(n C.int) *C.char {
	b, err := historyJSON(int(n))
	if err != nil {
		return nil
	}
	return C.CString(string(b))
}

// MidgardGet is the bytes of copy seq, their length in size and their type
// in mime, which the caller frees too; NULL when there is no such copy.
//
//export MidgardGet
func MidgardGet(seq C.uint64_t, mime **C.char, size *C.int64_t) unsafe.Pointer {
	t, data, err := get(uint64(seq))
	if err != nil {
		return nil
	}
	*mime = C.CString(t)
	*size = C.int64_t(len(data))
	return C.CBytes(data)
}

// MidgardStatus is the engine's state as JSON: {"configured","server",
// "signed_in","running","online","device","name"}.
//
//export MidgardStatus
func MidgardStatus() *C.char {
	b, _ := jsonOf(statusNow())
	return C.CString(string(b))
}

// MidgardLogin signs the Mac in, calling open with the link to approve at,
// and returns once it is approved: call it off the main thread.
//
//export MidgardLogin
func MidgardLogin(open C.MidgardOpen) *C.char {
	return cerr(login(func(link string) error {
		l := C.CString(link)
		defer C.free(unsafe.Pointer(l))
		C.midgardCallOpen(open, l)
		return nil
	}))
}

// MidgardLogout forgets the Mac's sign-in.
//
//export MidgardLogout
func MidgardLogout() *C.char { return cerr(logout()) }

// MidgardShare publishes data at a link, returned as {"url"} or {"error"}.
//
//export MidgardShare
func MidgardShare(mime *C.char, data unsafe.Pointer, size C.int64_t) *C.char {
	url, err := share(C.GoString(mime), C.GoBytes(data, C.int(size)))
	out := map[string]string{"url": url}
	if err != nil {
		out = map[string]string{"error": err.Error()}
	}
	b, _ := jsonOf(out)
	return C.CString(string(b))
}

// MidgardPairShow leaves this Mac's key for another device, and returns the
// pairing code and link as {"code", "link"}, or {"error"}.
//
//export MidgardPairShow
func MidgardPairShow() *C.char {
	code, link, err := pairShow()
	out := map[string]string{"code": code, "link": link}
	if err != nil {
		out = map[string]string{"error": err.Error()}
	}
	b, _ := jsonOf(out)
	return C.CString(string(b))
}

// MidgardPairJoin takes the person's key with a pairing code.
//
//export MidgardPairJoin
func MidgardPairJoin(code *C.char) *C.char { return cerr(pairJoin(C.GoString(code))) }

// MidgardSetBridge switches the Mac's Shortcuts bridge on (1) or off (0):
// it hands iPhone Shortcuts copies in the clear (specs/redesign.md §11).
//
//export MidgardSetBridge
func MidgardSetBridge(on C.int) *C.char { return cerr(setBridge(on != 0)) }

// MidgardFree gives back what the engine returned.
//
//export MidgardFree
func MidgardFree(p unsafe.Pointer) { C.free(p) }
