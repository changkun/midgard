// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import ImageIO
import ServiceManagement

/// A device's name as people say it: without the .local a Mac adds.
func shortName(_ name: String) -> String {
    name.hasSuffix(".local") ? String(name.dropLast(6)) : name
}

/// The app's state: the engine's status and history, and what the Mac's
/// clipboard does with them.
@MainActor
final class Model: ObservableObject {
    @Published var status = Status()
    @Published var history: [HistoryItem] = []
    @Published var problem: String?   // why it is not syncing, in words
    @Published private(set) var signInLink: URL?    // while a sign-in waits for approval
    @Published var lastShare: URL?
    /// The copy just put back on the clipboard, for a moment, to show it.
    @Published private(set) var justCopied: String?

    /// Paused, midgard neither reads this Mac's clipboard nor writes to it.
    @Published var paused: Bool {
        didSet { UserDefaults.standard.set(paused, forKey: "paused") }
    }

    let pasteboard: LocalClipboard
    private var hotkey: Hotkey?
    private var timer: Timer?

    init(pasteboard: LocalClipboard = LocalClipboard()) {
        self.pasteboard = pasteboard
        paused = UserDefaults.standard.bool(forKey: "paused")
    }

    /// Starts syncing, if the server is set.
    func start() {
        Engine.onChanged = { [weak self] mime, data in
            guard let self, !self.paused else { return }
            self.pasteboard.write(mime: mime, data: data)
            self.refresh()
        }
        pasteboard.onCopy = { [weak self] mime, data in
            guard let self, !self.paused else { return }
            do { try Engine.copy(mime: mime, data: data) } catch { self.problem = "\(error)" }
            self.refresh()
        }
        pasteboard.start()
        hotkey = Hotkey { [weak self] in self?.shareClipboard() }
        timer = Timer.scheduledTimer(withTimeInterval: 2, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.refresh() }
        }
        startEngine()
        checkLoginItem()
    }

    private func startEngine() {
        do {
            try Engine.start()
            problem = nil
        } catch Engine.StartError.noServer {
            problem = "Set your Midgard server to start."
        } catch Engine.StartError.running {
            problem = "mg daemon already syncs this Mac. Stop it (mg daemon stop, then mg daemon uninstall) to use the app instead."
        } catch {
            problem = "Cannot start: \(error)"
        }
        refresh()
    }

    /// Whether refresh asks the engine; views ask on appearing, which a
    /// preview of sample data must not.
    var live = true

    /// Asks the engine what changed. It runs every 2 seconds, so it sets only
    /// what differs: setting a published value, even to what it was, draws
    /// every view of the model again.
    func refresh() {
        guard live else { return }
        let s = Engine.status()
        if s != status { status = s }
        guard s.running else { return }
        let h = Engine.history()
        if h != history { history = h }
        var p = problem
        if problem == nil || problem?.hasPrefix("Cannot") == false, s.configured, !s.signedIn {
            p = "Sign in to sync."
        } else if s.signedIn, problem == "Sign in to sync." {
            p = nil
        }
        if p != problem { problem = p }
    }

    /// Sets the server, and starts.
    func setServer(_ server: String) {
        let s = server.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !s.isEmpty else { return }
        do {
            try Engine.setup(server: s)
            Engine.stop()
            startEngine()
        } catch {
            problem = "Cannot set the server: \(error)"
        }
    }

    func signIn() {
        Task {
            do {
                try await Engine.login { url in
                    self.signInLink = url
                    NSWorkspace.shared.open(url)
                }
                signInLink = nil
                problem = nil
            } catch {
                signInLink = nil
                problem = "Signing in failed: \(error)"
            }
            refresh()
        }
    }

    func signOut() {
        try? Engine.logout()
        refresh()
    }

    /// Makes an older copy the clipboard again, on this Mac and every device:
    /// written here, it is a copy like any other.
    func use(_ item: HistoryItem) {
        guard !item.waiting, let (mime, data) = Engine.get(item.seq) else { return }
        pasteboard.write(mime: mime, data: data)
        if !paused { try? Engine.copy(mime: mime, data: data) }
        justCopied = item.id
        Task { @MainActor in
            try? await Task.sleep(for: .seconds(1.2))
            if justCopied == item.id { justCopied = nil }
        }
        refresh()
    }

    /// Where midgard stands, for the views to say.
    enum Phase: Equatable {
        case needsServer          // the first start
        case needsSignIn
        case blocked(String)      // it cannot run, and why
        case ready
    }

    var phase: Phase {
        if !status.configured { return .needsServer }
        if !status.running { return .blocked(problem ?? "Midgard is not running.") }
        if !status.signedIn { return .needsSignIn }
        return .ready
    }

    /// The devices copies came from, the most recent first.
    var devices: [String] {
        var seen = Set<String>(), out: [String] = []
        for item in history where !item.origin.isEmpty && seen.insert(item.origin).inserted {
            out.append(item.origin)
        }
        return out
    }

    private let thumbnails = NSCache<NSNumber, NSImage>()

    /// A small image of a copy that is one, made once, away from the main
    /// thread: decoding a screenshot takes longer than a frame of scrolling.
    func thumbnail(_ item: HistoryItem) async -> NSImage? {
        guard item.isImage, !item.waiting else { return nil }
        let key = NSNumber(value: item.seq)
        if let cached = thumbnails.object(forKey: key) { return cached }
        let seq = item.seq
        let small = await Task.detached(priority: .userInitiated) { () -> CGImage? in
            guard let (_, data) = Engine.get(seq),
                  let source = CGImageSourceCreateWithData(data as CFData, nil) else { return nil }
            return CGImageSourceCreateThumbnailAtIndex(source, 0, [
                kCGImageSourceCreateThumbnailFromImageAlways: true,
                kCGImageSourceCreateThumbnailWithTransform: true,
                kCGImageSourceThumbnailMaxPixelSize: 128, // a list's tile, on a Retina screen
            ] as CFDictionary)
        }.value
        guard let small else { return nil }
        let image = NSImage(cgImage: small, size: .zero)
        thumbnails.setObject(image, forKey: key)
        return image
    }

    /// The app's version, as the bundle says it.
    var version: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "dev"
    }

    func delete(_ item: HistoryItem) {
        try? Engine.delete(item.seq)
        refresh()
    }

    func clearHistory() {
        try? Engine.clear()
        refresh()
    }

    /// Shares what is on the clipboard at a link, and puts the link there.
    func shareClipboard() {
        guard let (mime, data) = pasteboard.read() else {
            problem = "There is nothing on the clipboard to share."
            return
        }
        do {
            let url = try Engine.share(mime: mime, data: data)
            lastShare = url
            pasteboard.write(mime: "text", data: Data(url.absoluteString.utf8))
            if !paused { try? Engine.copy(mime: "text", data: Data(url.absoluteString.utf8)) }
        } catch {
            problem = "Cannot share: \(error)"
        }
        refresh()
    }

    /// Whether the app starts when you log in. Asking macOS is a round trip
    /// to another process, of tens of milliseconds, too slow for a view to do
    /// each time it draws: it is asked in checkLoginItem, and kept.
    var startsAtLogin: Bool {
        get { loginItem }
        set {
            do {
                if newValue { try SMAppService.mainApp.register() } else { try SMAppService.mainApp.unregister() }
            } catch {
                problem = "Cannot change starting at login: \(error.localizedDescription)"
            }
            checkLoginItem()
        }
    }
    @Published private var loginItem = false

    /// Asks macOS whether the app starts at login, as one can change that in
    /// System Settings too; the views ask when they appear.
    func checkLoginItem() {
        guard live else { return }
        Task {
            let on = await Task.detached(priority: .utility) { SMAppService.mainApp.status == .enabled }.value
            if on != loginItem { loginItem = on }
        }
    }

    /// A word on what Midgard does now, and its colour.
    var state: (label: String, color: NSColor) {
        switch phase {
        case .needsServer: return ("Set up", .systemOrange)
        case .needsSignIn: return ("Sign in", .systemOrange)
        case .blocked: return ("Stopped", .systemRed)
        case .ready:
            if paused { return ("Paused", .systemYellow) }
            return status.online ? ("Syncing", .systemGreen) : ("Offline", .systemGray)
        }
    }

    /// A line on what Midgard does now.
    var summary: String {
        if let problem, phase != .ready { return problem }
        if paused { return "Paused: Midgard leaves this Mac's clipboard alone." }
        if status.online { return "Syncing as \(shortName(status.name ?? "this Mac"))." }
        return "Offline: copies wait until the server is back."
    }
}
