// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

#import <Foundation/Foundation.h>
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h> // for keyboard hotkey

unsigned int clipboard_read_string(void **out) {
	NSData *data = [[NSPasteboard generalPasteboard] dataForType:NSPasteboardTypeString];
	if (data == nil) {
		return 0;
	}
	NSUInteger siz = [data length];
	*out = malloc(siz);
	[data getBytes: *out length: siz];
	return siz;
}

unsigned int clipboard_read_image(void **out) {
	NSData *data = [[NSPasteboard generalPasteboard] dataForType:NSPasteboardTypePNG];
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
	return [[NSPasteboard generalPasteboard] changeCount];
}

// -------- global hotkey shortcut handling ----------

extern void go_hotkey_callback(void* handler);

static OSStatus _hotkey_handler(EventHandlerCallRef nextHandler, EventRef theEvent, void *userData) {
	EventHotKeyID k;
	GetEventParameter(theEvent, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(k), NULL, &k);
	if (k.id == 1) {
		void *go_hotkey_handler = userData;
		go_hotkey_callback(go_hotkey_handler);
	}
	return noErr;
}

// register_hotkey registers a global system hotkey for callbacks.
//
// example: https://snippets.aktagon.com/snippets/361-registering-global-hot-keys-with-cocoa-and-objective-c
// keycode: http://macbiblioblog.blogspot.com/2014/12/key-codes-for-function-and-special-keys.html
// call go from c: https://stackoverflow.com/questions/37157379/passing-function-pointer-to-the-c-code-using-cgo
int register_hotkey(void* go_hotkey_handler) {
	EventHotKeyID hotKeyID;
	EventTypeSpec eventType;

	eventType.eventClass = kEventClassKeyboard;
	eventType.eventKind = kEventHotKeyPressed;

	hotKeyID.signature = 'htk1';
	hotKeyID.id = 1;

	InstallApplicationEventHandler(&_hotkey_handler, 1, &eventType, go_hotkey_handler, NULL);

	// Register the event hot key
	// modifiers: cmdKey, controlKey, optionKey, shiftKey.
	// keycode s == 1
	EventHotKeyRef hotKeyRef;
	OSStatus s = RegisterEventHotKey(1, controlKey+optionKey, hotKeyID,
		GetApplicationEventTarget(), 0, &hotKeyRef);
	if (s != noErr) {
		return -1;
	}
	NSLog(@"hotkey registered");
	return 0;
}


// The following three lines of code must run on the main thread.
// Don't ask why. This is really bad. Go must handle this using the
// pkg/mainthread.
//
// inspired from here: https://github.com/cehoffman/dotfiles/blob/4be8e893517e970d40746a9bdc67fe5832dd1c33/os/mac/iTerm2HotKey.m
void run_shared_application() {
	[NSApplication sharedApplication];
	[NSApp disableRelaunchOnLogin];
	[NSApp run];
}
