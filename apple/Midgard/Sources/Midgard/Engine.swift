// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import CMidgard
import Foundation

/// The sync engine: midgard's Go engine, the same as mg daemon's, linked
/// as a C library (apple/engine). Every call is quick but Login, which waits
/// for the sign-in to be approved.
enum Engine {
    /// Why the engine did not start.
    enum StartError: Error, Equatable {
        case noServer          // the server is not set yet
        case running           // mg daemon, or another app, syncs this Mac
        case failed(String)
    }

    /// Called on the main thread with a copy from another device, which
    /// belongs on the clipboard.
    static var onChanged: ((String, Data) -> Void)?
    private static var onOpen: ((URL) -> Void)?

    static func start() throws {
        var code: Int32 = 0
        let err = take(MidgardStart({ mime, data, size in
            guard let mime, let data else { return }
            let m = String(cString: mime)
            let d = Data(bytes: data, count: Int(size)) // the bytes are the engine's only during the call
            DispatchQueue.main.async { Engine.onChanged?(m, d) }
        }, &code))
        switch code {
        case 0: return
        case 1: throw StartError.noServer
        case 2: throw StartError.running
        default: throw StartError.failed(err ?? "unknown")
        }
    }

    static func stop() { MidgardStop() }

    static func setup(server: String) throws {
        try check(server.withCString { MidgardSetup(UnsafeMutablePointer(mutating: $0)) })
    }

    static func copy(mime: String, data: Data) throws {
        try check(withBytes(mime, data) { m, p, n in MidgardCopy(m, p, n) })
    }

    static func delete(_ seq: UInt64) throws { try check(MidgardDelete(seq)) }
    static func clear() throws { try check(MidgardClear()) }

    static func history(limit: Int = 200) -> [HistoryItem] {
        guard let json = take(MidgardHistory(Int32(limit))) else { return [] }
        return (try? JSONDecoder().decode([HistoryItem].self, from: Data(json.utf8))) ?? []
    }

    /// The bytes of a copy in the history, and their type.
    static func get(_ seq: UInt64) -> (mime: String, data: Data)? {
        var mime: UnsafeMutablePointer<CChar>?
        var size: Int64 = 0
        guard let p = MidgardGet(seq, &mime, &size) else { return nil }
        defer { MidgardFree(p) }
        let data = Data(bytes: p, count: Int(size))
        return (take(mime) ?? "", data)
    }

    static func status() -> Status {
        guard let json = take(MidgardStatus()),
              let s = try? JSONDecoder().decode(Status.self, from: Data(json.utf8)) else { return Status() }
        return s
    }

    /// Signs the Mac in: open is called with the link to approve at, and it
    /// returns once approved.
    static func login(open: @escaping (URL) -> Void) async throws {
        onOpen = open
        let err: String? = await withCheckedContinuation { done in
            DispatchQueue.global().async {
                let e = take(MidgardLogin({ link in
                    guard let link, let url = URL(string: String(cString: link)) else { return }
                    DispatchQueue.main.async { Engine.onOpen?(url) }
                }))
                done.resume(returning: e)
            }
        }
        if let err { throw StartError.failed(err) }
    }

    static func logout() throws { try check(MidgardLogout()) }

    /// Publishes data at a link.
    static func share(mime: String, data: Data) throws -> URL {
        let json = take(withBytes(mime, data) { m, p, n in MidgardShare(m, p, n) }) ?? "{}"
        let out = (try? JSONDecoder().decode([String: String].self, from: Data(json.utf8))) ?? [:]
        if let url = out["url"].flatMap(URL.init(string:)), !url.absoluteString.isEmpty { return url }
        throw StartError.failed(out["error"] ?? "cannot share")
    }

    // What the engine returns is the caller's, to give back.
    private static func take(_ p: UnsafeMutablePointer<CChar>?) -> String? {
        guard let p else { return nil }
        defer { MidgardFree(p) }
        return String(cString: p)
    }

    private static func check(_ p: UnsafeMutablePointer<CChar>?) throws {
        if let err = take(p) { throw StartError.failed(err) }
    }

    private static func withBytes<T>(_ mime: String, _ data: Data,
                                     _ f: (UnsafeMutablePointer<CChar>, UnsafeMutableRawPointer?, Int64) -> T) -> T {
        var data = data
        return mime.withCString { m in
            data.withUnsafeMutableBytes { b in
                f(UnsafeMutablePointer(mutating: m), b.baseAddress, Int64(b.count))
            }
        }
    }
}

/// A copy in the history, as the engine lists it.
struct HistoryItem: Decodable, Identifiable, Equatable {
    var seq: UInt64
    var time: Int64
    var origin: String
    var mime: String
    var size: Int
    var preview: String?
    var waiting: Bool

    var id: String { waiting ? "waiting-\(time)" : "\(seq)" }
    var date: Date { Date(timeIntervalSince1970: Double(time) / 1000) }
    var isImage: Bool { mime == "image/png" }
}

/// What the engine says of itself.
struct Status: Decodable, Equatable {
    var configured = false
    var server: String?
    var signedIn = false
    var running = false
    var online = false
    var device: String?
    var name: String?

    enum CodingKeys: String, CodingKey {
        case configured, server, running, online, device, name
        case signedIn = "signed_in"
    }
}
