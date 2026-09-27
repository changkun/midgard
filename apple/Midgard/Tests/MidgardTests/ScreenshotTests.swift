// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
@testable import Midgard
import SwiftUI
import XCTest

/// Photographs the app's windows on screen, with sample data, for the README
/// and the web page:
///
///	MIDGARD_SCREENSHOTS=/tmp/shots swift test --filter ScreenshotTests
///
/// Unlike SnapshotTests, which draws views offscreen, it shows real windows
/// for a few seconds, so that they have their frames, materials and shadows,
/// over a backdrop of its own, and needs Screen Recording for the terminal.
/// Without MIDGARD_SCREENSHOTS it is skipped.
@MainActor
final class ScreenshotTests: XCTestCase {
    private var dir: URL!
    private var backdrop: NSWindow?

    override func setUp() async throws {
        guard let path = ProcessInfo.processInfo.environment["MIDGARD_SCREENSHOTS"] else {
            throw XCTSkip("MIDGARD_SCREENSHOTS names no directory")
        }
        dir = URL(fileURLWithPath: path)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        NSApplication.shared.setActivationPolicy(.accessory)
        let icon = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .appendingPathComponent("../../Icon/AppIcon.icns")
        NSApp.applicationIconImage = NSImage(contentsOf: icon)
    }

    override func tearDown() async throws {
        backdrop?.orderOut(nil)
        backdrop = nil
    }

    /// The story the pictures tell: a flight's details, copied on this Mac
    /// just now, over what the person's devices copied before.
    private func model(copied: Bool, dark: Bool) -> Model {
        let m = Model(pasteboard: LocalClipboard(NSPasteboard(name: .init("midgard-screenshots"))))
        m.live = false
        m.status = Status(configured: true, server: "https://changkun.de", signedIn: true, running: true,
                          online: true, device: "a1", name: "MacBook-Pro", sealing: true)
        let now = Date().timeIntervalSince1970 * 1000
        let minute = 60_000.0
        var items = [
            HistoryItem(seq: 57, time: Int64(now - 2 * minute), origin: "MacBook-Pro", mime: "text", size: 116,
                        preview: "func (r *relay) number(ctx context.Context, rm *room, from *link, origin string, f wire.Frame) (wire.Frame, error) {", waiting: false),
            HistoryItem(seq: 56, time: Int64(now - 4 * minute), origin: "iPhone", mime: "image/png", size: 248_512,
                        preview: nil, waiting: false),
            HistoryItem(seq: 55, time: Int64(now - 6 * minute), origin: "ubuntu-desktop", mime: "text", size: 62,
                        preview: "Meeting notes: ship the relay, then the Mac app, then the DMG.", waiting: false),
            HistoryItem(seq: 54, time: Int64(now - 8 * minute), origin: "iPhone", mime: "text", size: 35,
                        preview: "https://github.com/changkun/midgard", waiting: false),
            HistoryItem(seq: 53, time: Int64(now - 26 * 60 * minute), origin: "MacBook-Pro", mime: "text", size: 12,
                        preview: "hello, world", waiting: false),
        ]
        if copied {
            items.insert(HistoryItem(seq: 58, time: Int64(now - 3000), origin: "MacBook-Pro", mime: "text", size: 61,
                                     preview: "Flight LH 2023 · Munich → Berlin · Gate G24, boarding 18:05", waiting: false), at: 0)
        }
        m.history = items
        m.thumbnails.setObject(photo(), forKey: 56)
        return m
    }

    /// A photo's stand-in: a lake at dusk under mountains.
    private func photo() -> NSImage {
        NSImage(size: NSSize(width: 128, height: 84), flipped: true) { r in
            NSGradient(colors: [NSColor(srgbRed: 0.17, green: 0.23, blue: 0.56, alpha: 1),
                                NSColor(srgbRed: 0.88, green: 0.45, blue: 0.61, alpha: 1),
                                NSColor(srgbRed: 1, green: 0.77, blue: 0.54, alpha: 1)])?.draw(in: r, angle: 90)
            NSColor(srgbRed: 1, green: 0.89, blue: 0.69, alpha: 1).setFill()
            NSBezierPath(ovalIn: NSRect(x: 82, y: 40, width: 16, height: 16)).fill()
            let hills = NSBezierPath()
            hills.move(to: NSPoint(x: 0, y: 60))
            for (x, y) in [(20, 40), (32, 50), (50, 30), (68, 52), (80, 42), (98, 55), (112, 38), (128, 50), (128, 84), (0, 84)] {
                hills.line(to: NSPoint(x: x, y: y))
            }
            hills.close()
            NSColor(srgbRed: 0.29, green: 0.25, blue: 0.53, alpha: 1).setFill()
            hills.fill()
            return true
        }
    }

    /// A screen-wide window behind the ones photographed, so that their
    /// materials show its colours, not whatever is on the screen.
    private func showBackdrop(dark: Bool) {
        backdrop?.orderOut(nil)
        let screen = NSScreen.main!.frame
        let w = NSWindow(contentRect: screen, styleMask: [.borderless], backing: .buffered, defer: false)
        w.level = .normal
        w.isReleasedWhenClosed = false
        let v = NSView(frame: screen)
        v.wantsLayer = true
        v.layer!.addSublayer(wallpaper(dark: dark, frame: v.bounds))
        w.contentView = v
        w.orderFrontRegardless()
        backdrop = w
    }

    /// The backdrop's colours: the web page's night sky, or a light morning.
    private func wallpaper(dark: Bool, frame: NSRect) -> CAGradientLayer {
        let g = CAGradientLayer()
        g.frame = frame
        g.colors = dark
            ? [NSColor(srgbRed: 0.05, green: 0.07, blue: 0.2, alpha: 1).cgColor, NSColor(srgbRed: 0.18, green: 0.13, blue: 0.36, alpha: 1).cgColor]
            : [NSColor(srgbRed: 0.84, green: 0.89, blue: 0.99, alpha: 1).cgColor, NSColor(srgbRed: 0.95, green: 0.87, blue: 0.95, alpha: 1).cgColor]
        return g
    }

    /// Shows window a moment, photographs it with its shadow, and closes it.
    private func photograph(_ window: NSWindow, _ name: String, prepare: ((NSWindow) -> Void)? = nil) throws {
        window.isReleasedWhenClosed = false
        window.level = .floating
        window.center()
        // active, as a window one works in looks: coloured buttons, an accent selection
        NSApp.activate(ignoringOtherApps: true)
        window.makeKeyAndOrderFront(nil)
        window.orderFrontRegardless()
        RunLoop.main.run(until: Date().addingTimeInterval(0.8))
        prepare?(window)
        RunLoop.main.run(until: Date().addingTimeInterval(1.2))
        let out = dir.appendingPathComponent(name + ".png")
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
        p.arguments = ["-x", "-l", String(window.windowNumber), out.path]
        try p.run()
        p.waitUntilExit()
        window.orderOut(nil)
        XCTAssertEqual(p.terminationStatus, 0, "screencapture of \(name)")
        XCTAssertTrue(FileManager.default.fileExists(atPath: out.path), "no picture of \(name): is Screen Recording allowed?")
    }

    func testPopover() throws {
        for dark in [false, true] {
            showBackdrop(dark: dark)
            for copied in [false, true] {
                let size = NSSize(width: 360, height: 480)
                let panel = NSPanel(contentRect: NSRect(origin: .zero, size: size), styleMask: [.borderless, .nonactivatingPanel],
                                    backing: .buffered, defer: false)
                panel.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
                panel.isOpaque = false
                panel.backgroundColor = .clear
                panel.hasShadow = true
                // as a menu bar extra's window is: a material, rounded, over
                // the wallpaper, which it holds itself, as a photograph of a
                // window alone shows nothing behind it for the material
                let bounds = NSRect(origin: .zero, size: size)
                let content = NSView(frame: bounds)
                content.wantsLayer = true
                content.layer!.cornerRadius = 12
                content.layer!.masksToBounds = true
                content.layer!.addSublayer(wallpaper(dark: dark, frame: bounds))
                let material = NSVisualEffectView(frame: bounds)
                material.material = .menu
                material.blendingMode = .withinWindow
                material.state = .active
                content.addSubview(material)
                let host = NSHostingView(rootView: PopoverView(model: model(copied: copied, dark: dark)).frame(width: size.width, height: size.height))
                host.frame = bounds
                content.addSubview(host)
                panel.contentView = content
                try photograph(panel, "popover" + (copied ? "-copied" : "") + (dark ? "-dark" : ""))
            }
        }
    }

    func testHistoryWindow() throws {
        for dark in [false, true] {
            showBackdrop(dark: dark)
            let controller = NSHostingController(rootView: HistoryWindow(model: model(copied: true, dark: dark)))
            controller.sceneBridgingOptions = [.toolbars, .title]
            let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 1000, height: 600),
                                  styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
                                  backing: .buffered, defer: false)
            window.contentViewController = controller
            window.setContentSize(NSSize(width: 1000, height: 600))
            window.toolbarStyle = .unified
            window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
            try photograph(window, "history" + (dark ? "-dark" : "")) { window in
                // select the newest copy, so the detail shows it whole
                let count = 6
                if let list = tables(in: window.contentView!).first(where: { $0.numberOfRows == count }) {
                    list.selectRowIndexes(IndexSet(integer: 0), byExtendingSelection: false)
                }
            }
        }
    }
}

/// The table views under v, in order: a split view's sidebar, then its list.
@MainActor
private func tables(in v: NSView) -> [NSTableView] {
    var found: [NSTableView] = []
    if let t = v as? NSTableView { found.append(t) }
    for sub in v.subviews { found += tables(in: sub) }
    return found
}
