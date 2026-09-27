// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
@testable import Midgard
import SwiftUI
import XCTest

/// Renders the app's views with sample data into PNGs, to look at them:
///
///	MIDGARD_SNAPSHOTS=/tmp/shots swift test --filter SnapshotTests
///
/// Without MIDGARD_SNAPSHOTS it renders them and checks only that they draw.
@MainActor
final class SnapshotTests: XCTestCase {
    private func model(_ phase: String = "ready") -> Model {
        let m = Model(pasteboard: LocalClipboard(NSPasteboard(name: .init("midgard-snapshots"))))
        m.live = false // sample data, not the engine's
        let now = Date().timeIntervalSince1970 * 1000
        switch phase {
        case "server":
            m.status = Status()
        case "signin":
            m.status = Status(configured: true, server: "https://changkun.de", signedIn: false, running: true)
        case "pairing":
            m.status = Status(configured: true, server: "https://changkun.de", signedIn: true, running: true, needsPairing: true)
        case "notonlist":
            m.status = Status(configured: true, server: "https://changkun.de", signedIn: true, running: true, notOnList: true)
        default:
            m.status = Status(configured: true, server: "https://changkun.de", signedIn: true, running: true,
                              online: true, device: "a1", name: "Changkun-MacBook-Pro.local")
            m.history = [
                HistoryItem(seq: 0, time: Int64(now - 4000), origin: "", mime: "text", size: 38,
                            preview: "https://changkun.de/midgard/s/fboVP8u4", waiting: true),
                HistoryItem(seq: 57, time: Int64(now - 90_000), origin: "Changkun-MacBook-Pro.local", mime: "text", size: 142,
                            preview: "func (r *relay) number(ctx context.Context, rm *room, from *link, origin string, f wire.Frame) (wire.Frame, error) {", waiting: false),
                HistoryItem(seq: 56, time: Int64(now - 600_000), origin: "iPhone", mime: "image/png", size: 248_512,
                            preview: nil, waiting: false),
                HistoryItem(seq: 55, time: Int64(now - 3_600_000), origin: "ubuntu-desktop", mime: "text", size: 61,
                            preview: "Meeting notes: ship the relay, then the Mac app, then the DMG.", waiting: false),
                HistoryItem(seq: 54, time: Int64(now - 7_200_000), origin: "iPhone", mime: "text", size: 29,
                            preview: "https://github.com/changkun/midgard", waiting: false),
                HistoryItem(seq: 53, time: Int64(now - 86_400_000), origin: "Changkun-MacBook-Pro.local", mime: "text", size: 12,
                            preview: "hello, world", waiting: false),
            ]
        }
        return m
    }

    private func render<V: View>(_ name: String, _ view: V, size: CGSize, dark: Bool = false) throws {
        let host = NSHostingView(rootView: view.frame(width: size.width, height: size.height))
        host.frame = NSRect(origin: .zero, size: size)
        let window = NSWindow(contentRect: host.frame, styleMask: [.borderless], backing: .buffered, defer: false)
        window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
        window.contentView = host
        host.layoutSubtreeIfNeeded()
        RunLoop.main.run(until: Date().addingTimeInterval(0.2))
        let rep = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
        host.cacheDisplay(in: host.bounds, to: rep)
        let png = try XCTUnwrap(rep.representation(using: .png, properties: [:]))
        XCTAssertGreaterThan(png.count, 1000, "\(name) drew nothing")
        if let dir = ProcessInfo.processInfo.environment["MIDGARD_SNAPSHOTS"] {
            try FileManager.default.createDirectory(atPath: dir, withIntermediateDirectories: true)
            try png.write(to: URL(fileURLWithPath: dir).appendingPathComponent(name + (dark ? "-dark" : "") + ".png"))
        }
    }

    func testPopover() throws {
        for dark in [false, true] {
            try render("popover", PopoverView(model: model()).background(Color(nsColor: .windowBackgroundColor)), size: CGSize(width: 360, height: 480), dark: dark)
        }
        try render("popover-setup", PopoverView(model: model("server")).background(Color(nsColor: .windowBackgroundColor)), size: CGSize(width: 360, height: 480))
        try render("popover-signin", PopoverView(model: model("signin")).background(Color(nsColor: .windowBackgroundColor)), size: CGSize(width: 360, height: 480))
        try render("popover-pairing", PopoverView(model: model("pairing")).background(Color(nsColor: .windowBackgroundColor)), size: CGSize(width: 360, height: 480))
        try render("popover-notonlist", PopoverView(model: model("notonlist")).background(Color(nsColor: .windowBackgroundColor)), size: CGSize(width: 360, height: 480))
    }

    func testPairingCode() throws {
        let link = try XCTUnwrap(URL(string: "https://changkun.de/midgard/#pair=7K2M9QXRA4BC8DEFGH1JKMNPQR"))
        try render("pairing-code", PairingCode(code: "7K2M-9QXR-A4BC-8DEF-GH1J-KMNP-QR", link: link).padding().background(Color(nsColor: .windowBackgroundColor)),
                   size: CGSize(width: 420, height: 180))
        XCTAssertNotNil(qrCode(link.absoluteString), "no QR for the pairing link")
    }

    func testHistoryWindow() throws {
        try render("history", HistoryWindow(model: model()), size: CGSize(width: 900, height: 560))
    }

    func testSettings() throws {
        try render("settings", SettingsView(model: model()), size: CGSize(width: 460, height: 320))
    }

    func testWelcome() throws {
        try render("welcome", WelcomeView(model: model("server")) {}, size: CGSize(width: 420, height: 400))
    }
}
