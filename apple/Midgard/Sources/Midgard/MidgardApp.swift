// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import SwiftUI

/// midgard on the Mac: in the menu bar, with a window for the history. It is
/// what mg daemon is elsewhere, and replaces it here: one of them syncs a
/// device at a time.
@main
struct MidgardApp: App {
    @NSApplicationDelegateAdaptor private var delegate: AppDelegate

    var body: some Scene {
        MenuBarExtra("midgard", systemImage: "doc.on.clipboard") {
            MenuView(model: delegate.model)
        }
        Window("History", id: "history") {
            HistoryView(model: delegate.model).frame(minWidth: 640, minHeight: 420)
        }
        Window("midgard Settings", id: "settings") {
            SettingsView(model: delegate.model)
        }
        .windowResizability(.contentSize)
    }
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    let model = Model()
    private var welcome: NSWindow? // kept, or it goes as soon as it opens

    func applicationDidFinishLaunching(_: Notification) {
        model.start()
        // The first time, ask for the server.
        if !model.status.configured {
            let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 440, height: 380),
                                  styleMask: [.titled, .closable], backing: .buffered, defer: false)
            window.contentView = NSHostingView(rootView: SettingsView(model: model))
            window.title = "Welcome to midgard"
            window.isReleasedWhenClosed = false
            window.center()
            window.makeKeyAndOrderFront(nil)
            welcome = window
            NSApp.activate(ignoringOtherApps: true)
        }
    }

    func applicationWillTerminate(_: Notification) {
        Engine.stop()
    }
}
