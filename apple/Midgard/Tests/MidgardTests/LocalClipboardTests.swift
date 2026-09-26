// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
@testable import Midgard
import XCTest

/// The pasteboard's tests use one of their own, never the Mac's clipboard.
final class PasteboardTests: XCTestCase {
    private var board: NSPasteboard!
    private var watcher: LocalClipboard!
    private var copies: [(String, Data)] = []

    override func setUp() {
        board = NSPasteboard(name: .init("midgard-test-\(UUID().uuidString)"))
        watcher = LocalClipboard(board)
        copies = []
        watcher.onCopy = { [unowned self] in copies.append(($0, $1)) }
    }

    override func tearDown() { board.releaseGlobally() }

    func testTextIsSynced() {
        board.clearContents()
        board.setString("hello", forType: .string)
        watcher.poll()
        XCTAssertEqual(copies.count, 1)
        XCTAssertEqual(copies.first?.0, "text")
        XCTAssertEqual(copies.first.map { String(decoding: $0.1, as: UTF8.self) }, "hello")
        // nothing changed since
        watcher.poll()
        XCTAssertEqual(copies.count, 1)
    }

    /// A password manager marks a password, by nspasteboard.org's convention;
    /// it is never synced.
    func testSecretsAreNot() {
        for marker in LocalClipboard.secret {
            board.clearContents()
            board.declareTypes([.string, marker], owner: nil)
            board.setString("hunter2", forType: .string)
            board.setData(Data(), forType: marker)
            watcher.poll()
        }
        XCTAssertTrue(copies.isEmpty, "synced \(copies.count) secrets")
    }

    /// What midgard writes came from another device: it is not sent back.
    func testOwnWritesAreNotCopies() {
        watcher.write(mime: "text", data: Data("from the laptop".utf8))
        watcher.poll()
        XCTAssertTrue(copies.isEmpty)
        XCTAssertEqual(board.string(forType: .string), "from the laptop")
    }

    /// Images are synced as PNG, whatever the app that copied offered.
    func testImagesAsPNG() throws {
        let image = NSImage(size: NSSize(width: 4, height: 4))
        image.lockFocus()
        NSColor.red.drawSwatch(in: NSRect(x: 0, y: 0, width: 4, height: 4))
        image.unlockFocus()
        let tiff = try XCTUnwrap(image.tiffRepresentation)
        board.clearContents()
        board.setData(tiff, forType: .tiff)
        watcher.poll()
        XCTAssertEqual(copies.first?.0, "image/png")
        XCTAssertEqual(copies.first?.1.prefix(8), Data([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]))

        watcher.write(mime: "image/png", data: copies[0].1)
        XCTAssertNotNil(board.data(forType: .png))
    }
}

final class DecodingTests: XCTestCase {
    func testHistory() throws {
        let json = #"[{"seq":3,"time":1700000000000,"origin":"laptop","mime":"text","size":5,"preview":"hello","waiting":false},{"seq":0,"time":1700000001000,"origin":"","mime":"image/png","size":10,"waiting":true}]"#
        let items = try JSONDecoder().decode([HistoryItem].self, from: Data(json.utf8))
        XCTAssertEqual(items.count, 2)
        XCTAssertEqual(items[0].preview, "hello")
        XCTAssertEqual(items[0].date, Date(timeIntervalSince1970: 1_700_000_000))
        XCTAssertTrue(items[1].isImage && items[1].waiting)
        XCTAssertNotEqual(items[0].id, items[1].id)
    }

    func testStatus() throws {
        let json = #"{"configured":true,"server":"https://changkun.de","signed_in":true,"running":true,"online":false,"device":"ab","name":"laptop"}"#
        let s = try JSONDecoder().decode(Status.self, from: Data(json.utf8))
        XCTAssertEqual(s, Status(configured: true, server: "https://changkun.de", signedIn: true, running: true, online: false, device: "ab", name: "laptop"))
    }
}
