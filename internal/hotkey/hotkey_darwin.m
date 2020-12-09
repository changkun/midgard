// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>

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
