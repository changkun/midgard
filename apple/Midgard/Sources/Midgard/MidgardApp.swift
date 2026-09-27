// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import Combine
import SwiftUI

/// Midgard on the Mac: in the menu bar, with a window for the history. It is
/// what mg daemon is elsewhere, and replaces it here: one of them syncs a
/// device at a time.
@main
struct MidgardApp: App {
    @NSApplicationDelegateAdaptor private var delegate: AppDelegate

    var body: some Scene {
        MenuBarExtra {
            PopoverView(model: delegate.model)
        } label: {
            MenuBarIcon()
        }
        .menuBarExtraStyle(.window)
        Window("Midgard History", id: "history") {
            HistoryWindow(model: delegate.model)
        }
        Settings {
            SettingsView(model: delegate.model)
        }
    }
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    let model = Model()
    private var updates: Updates?
    private var welcome: NSWindow? // kept, or it goes as soon as it opens
    private var watches: Set<AnyCancellable> = []

    func applicationDidFinishLaunching(_: Notification) {
        model.start()
        followWindows()
        let updates = Updates()
        self.updates = updates
        model.updates = updates
        // The first time, a window says what Midgard is and sets it up:
        // afterwards it lives in the menu bar, which is easy to miss.
        if !model.status.configured || !model.status.signedIn {
            let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 420, height: 400),
                                  styleMask: [.titled, .closable], backing: .buffered, defer: false)
            window.contentView = NSHostingView(rootView: WelcomeView(model: model) { [weak self] in
                self?.welcome?.close()
            })
            window.title = "Welcome to Midgard"
            window.isReleasedWhenClosed = false
            window.center()
            window.makeKeyAndOrderFront(nil)
            welcome = window
            NSApp.activate(ignoringOtherApps: true)
            // Signing in happens in the browser, which stays in front: once it
            // is done, the welcome comes back, to say what is next.
            model.$status.map { [model] _ in model.phase }.removeDuplicates().dropFirst()
                .sink { [weak self] phase in
                    guard let window = self?.welcome, window.isVisible, phase != .needsServer, phase != .needsSignIn else { return }
                    window.makeKeyAndOrderFront(nil)
                    NSApp.activate(ignoringOtherApps: true)
                }
                .store(in: &watches)
        }
    }

    /// Midgard lives in the menu bar, in neither the Dock nor Command-Tab.
    /// While one of its windows is open (the welcome, the history, Settings)
    /// it is in both, so one can find the window again; once the last
    /// closes, it is in the menu bar alone.
    private func followWindows() {
        let center = NotificationCenter.default
        for name in [NSWindow.didBecomeKeyNotification, NSWindow.willCloseNotification, NSWindow.didMiniaturizeNotification] {
            center.publisher(for: name)
                // a closing window is still open until the change is done
                .receive(on: RunLoop.main)
                .sink { _ in
                    // the menu bar's own window is a panel, as are alerts: they do not count
                    let open = NSApp.windows.contains {
                        !($0 is NSPanel) && $0.styleMask.contains(.titled) && ($0.isVisible || $0.isMiniaturized)
                    }
                    let policy: NSApplication.ActivationPolicy = open ? .regular : .accessory
                    if NSApp.activationPolicy() != policy {
                        NSApp.setActivationPolicy(policy)
                    }
                }
                .store(in: &watches)
        }
    }

    func applicationWillTerminate(_: Notification) {
        Engine.stop()
    }
}

/// The ring and the peaks of the app's icon, in one colour, which the system
/// tints for a light or a dark menu bar. Built with swift build alone, the
/// app has no such image, and shows a symbol instead.
struct MenuBarIcon: View {
    var body: some View {
        if let image = NSImage(named: "MenuBarIcon") {
            let _ = (image.isTemplate = true)
            Image(nsImage: image)
        } else {
            Image(systemName: "doc.on.clipboard")
        }
    }
}
