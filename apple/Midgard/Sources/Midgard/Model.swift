// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import ServiceManagement

/// A device's name as people say it: without the .local a Mac adds.
func shortName(_ name: String) -> String {
    name.hasSuffix(".local") ? String(name.dropLast(6)) : name
}

/// The app's state: the engine's status and history, and what the Mac's
/// clipboard does with them.
@MainActor
final class Model: ObservableObject {
    @Published internal(set) var status = Status()
    @Published internal(set) var history: [HistoryItem] = []
    @Published internal(set) var problem: String?   // why it is not syncing, in words
    @Published private(set) var signInLink: URL?    // while a sign-in waits for approval
    @Published internal(set) var lastShare: URL?
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

    func refresh() {
        guard live else { return }
        status = Engine.status()
        if status.running {
            history = Engine.history()
            if problem == nil || problem?.hasPrefix("Cannot") == false, status.configured, !status.signedIn {
                problem = "Sign in to sync."
            } else if status.signedIn, problem == "Sign in to sync." {
                problem = nil
            }
        }
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

    /// A small image of a copy that is one, made once.
    func thumbnail(_ item: HistoryItem) -> NSImage? {
        guard item.isImage, !item.waiting else { return nil }
        if let cached = thumbnails.object(forKey: NSNumber(value: item.seq)) { return cached }
        guard let (_, data) = Engine.get(item.seq), let image = NSImage(data: data) else { return nil }
        thumbnails.setObject(image, forKey: NSNumber(value: item.seq))
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

    /// Whether the app starts when you log in.
    var startsAtLogin: Bool {
        get { SMAppService.mainApp.status == .enabled }
        set {
            do {
                if newValue { try SMAppService.mainApp.register() } else { try SMAppService.mainApp.unregister() }
            } catch {
                problem = "Cannot change starting at login: \(error.localizedDescription)"
            }
            objectWillChange.send()
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
