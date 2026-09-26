// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import Carbon.HIToolbox

/// The global hotkey, Ctrl+Option+S: share the clipboard at a link. Carbon's
/// hotkeys need no Accessibility permission, unlike an event tap.
final class Hotkey {
    private var ref: EventHotKeyRef?
    private var handler: EventHandlerRef?
    private static var fired: (() -> Void)?

    init?(action: @escaping () -> Void) {
        Hotkey.fired = action
        var spec = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))
        let status = InstallEventHandler(GetApplicationEventTarget(), { _, _, _ in
            DispatchQueue.main.async { Hotkey.fired?() }
            return noErr
        }, 1, &spec, nil, &handler)
        guard status == noErr else { return nil }
        let id = EventHotKeyID(signature: OSType(0x6D67_6473), id: 1) // "mgds"
        guard RegisterEventHotKey(UInt32(kVK_ANSI_S), UInt32(controlKey | optionKey), id,
                                  GetApplicationEventTarget(), 0, &ref) == noErr else { return nil }
    }

    deinit {
        if let ref { UnregisterEventHotKey(ref) }
        if let handler { RemoveEventHandler(handler) }
    }
}
