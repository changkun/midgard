// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import ServiceManagement

/// The app's state: the engine's status and history, and what the Mac's
/// clipboard does with them.
@MainActor
final class Model: ObservableObject {
    @Published private(set) var status = Status()
    @Published private(set) var history: [HistoryItem] = []
    @Published private(set) var problem: String?   // why it is not syncing, in words
    @Published private(set) var signInLink: URL?    // while a sign-in waits for approval
    @Published private(set) var lastShare: URL?

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
            problem = "Set your midgard server to start."
        } catch Engine.StartError.running {
            problem = "mg daemon already syncs this Mac. Stop it (mg daemon stop, then mg daemon uninstall) to use the app instead."
        } catch {
            problem = "Cannot start: \(error)"
        }
        refresh()
    }

    func refresh() {
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
        guard let (mime, data) = Engine.get(item.seq) else { return }
        pasteboard.write(mime: mime, data: data)
        if !paused { try? Engine.copy(mime: mime, data: data) }
        refresh()
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

    /// A line on what midgard does now.
    var summary: String {
        if let problem { return problem }
        if paused { return "Paused" }
        if !status.running { return "Not running" }
        return status.online ? "Syncing as \(status.name ?? "this Mac")" : "Offline; copies wait until the server is back"
    }
}
