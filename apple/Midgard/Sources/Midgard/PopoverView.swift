// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import SwiftUI

/// What opens from the menu bar: the recent copies, to search and put back,
/// and what Midgard does.
struct PopoverView: View {
    @ObservedObject var model: Model
    @Environment(\.openWindow) private var openWindow
    @Environment(\.openSettings) private var openSettings
    @State private var search = ""

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            switch model.phase {
            case .ready:
                searchField
                list
            case .needsServer:
                Setup(model: model)
            case .needsSignIn:
                SignInPrompt(model: model)
            case .needsPairing:
                PairPrompt(model: model)
            case .blocked(let why):
                Notice(symbol: "exclamationmark.triangle", title: "Midgard is not syncing", text: why)
            }
            Divider()
            footer
        }
        .frame(width: 360, height: 480)
        .onAppear {
            model.refresh()
            model.checkLoginItem()
        }
    }

    private var header: some View {
        HStack(spacing: 8) {
            Logo(size: 22)
            Text("Midgard").font(.headline)
            Spacer()
            StatusPill(model: model)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
    }

    private var searchField: some View {
        HStack(spacing: 6) {
            Image(systemName: "magnifyingglass").foregroundStyle(.secondary)
            TextField("Search your history", text: $search)
                .textFieldStyle(.plain)
            if !search.isEmpty {
                Button { search = "" } label: { Image(systemName: "xmark.circle.fill") }
                    .buttonStyle(.borderless).foregroundStyle(.secondary)
            }
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 7)
        .background(.quaternary.opacity(0.6), in: RoundedRectangle(cornerRadius: 8, style: .continuous))
        .padding(.horizontal, 12)
        .padding(.vertical, 10)
    }

    private var items: [HistoryItem] {
        let all = model.history
        guard !search.isEmpty else { return Array(all.prefix(50)) }
        return all.filter { ($0.preview ?? "").localizedCaseInsensitiveContains(search) || $0.origin.localizedCaseInsensitiveContains(search) }
    }

    @ViewBuilder private var list: some View {
        if items.isEmpty {
            Notice(symbol: search.isEmpty ? "doc.on.clipboard" : "magnifyingglass",
                   title: search.isEmpty ? "Nothing copied yet" : "No copy matches",
                   text: search.isEmpty ? "Copy something on any of your devices, and it shows up here." : "Try other words.")
        } else {
            ScrollView {
                LazyVStack(spacing: 2) {
                    ForEach(items) { item in
                        PopoverRow(item: item, model: model)
                    }
                }
                .padding(.horizontal, 6)
                .padding(.bottom, 6)
            }
        }
    }

    private var footer: some View {
        HStack(spacing: 2) {
            IconButton(name: model.paused ? "Resume Syncing" : "Pause Syncing",
                       symbol: model.paused ? "play.circle" : "pause.circle") { model.paused.toggle() }
            IconButton(name: "Share Clipboard at a Link (⌃⌥S)", symbol: "link") { model.shareClipboard() }
                .disabled(model.phase != .ready)
            IconButton(name: "Open History", symbol: "clock.arrow.circlepath") {
                openWindow(id: "history")
                NSApp.activate(ignoringOtherApps: true)
            }
            Spacer()
            if let url = model.lastShare {
                Button {
                    model.pasteboard.write(mime: "text", data: Data(url.absoluteString.utf8))
                } label: {
                    Label("Link copied", systemImage: "checkmark.circle.fill").font(.caption)
                }
                .buttonStyle(.borderless)
                .foregroundStyle(.green)
                .help(url.absoluteString)
            }
            Menu {
                Button("Settings…") {
                    openSettings()
                    NSApp.activate(ignoringOtherApps: true)
                }
                Toggle("Start at Login", isOn: Binding(get: { model.startsAtLogin }, set: { model.startsAtLogin = $0 }))
                if model.status.signedIn {
                    Button("Sign Out") { model.signOut() }
                }
                Divider()
                Button("Quit Midgard") { NSApp.terminate(nil) }
            } label: {
                Image(systemName: "gearshape")
            }
            .menuStyle(.borderlessButton)
            .menuIndicator(.hidden)
            .fixedSize()
            .help("Settings")
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 6)
    }
}

/// A row of the popover's list, lit under the pointer. It keeps whether it
/// is itself: as the list scrolls under the pointer, that changes every few
/// frames, and then only this row draws again, not the popover.
private struct PopoverRow: View {
    let item: HistoryItem
    @ObservedObject var model: Model
    @State private var hovered = false

    var body: some View {
        ItemRow(item: item, model: model)
            .padding(.horizontal, 10)
            .padding(.vertical, 7)
            .background(
                RoundedRectangle(cornerRadius: 8, style: .continuous)
                    .fill(hovered ? Color.accentColor.opacity(0.14) : .clear))
            .onHover { hovered = $0 }
            .onTapGesture { model.use(item) }
            .contextMenu {
                Button("Copy") { model.use(item) }.disabled(item.waiting)
                Button("Delete", role: .destructive) { model.delete(item) }.disabled(item.waiting)
            }
            .help(item.waiting ? "Waiting to reach the server" : "Click to put it on the clipboard")
    }
}

/// A picture, a title and a line, where a list would be.
struct Notice: View {
    let symbol: String
    let title: String
    let text: String
    var body: some View {
        VStack(spacing: 8) {
            Spacer()
            Image(systemName: symbol).font(.system(size: 30)).foregroundStyle(.tertiary)
            Text(title).font(.headline)
            Text(text).font(.callout).foregroundStyle(.secondary).multilineTextAlignment(.center)
            Spacer()
        }
        .padding(24)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

/// The first start, in the popover: the server.
struct Setup: View {
    @ObservedObject var model: Model
    @State private var server = "changkun.de"
    var body: some View {
        VStack(spacing: 14) {
            Spacer()
            Logo(size: 64)
            Text("Welcome to Midgard").font(.title3.weight(.semibold))
            Text("Your clipboard, and its history, on all your devices.")
                .font(.callout).foregroundStyle(.secondary).multilineTextAlignment(.center)
            // a label and a line that stay in sight, as a field's own label
            // shows only while it is empty, and this one starts filled
            VStack(alignment: .leading, spacing: 6) {
                Text("Your Midgard server").font(.callout.weight(.semibold))
                TextField("example.com", text: $server)
                    .textFieldStyle(.roundedBorder)
                    .onSubmit { model.setServer(server) }
                Text("The server your devices sync through: the address whoever runs it gave you. Next, you sign in, in your browser.")
                    .font(.caption).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            .frame(maxWidth: 280)
            Button("Continue") { model.setServer(server) }
                .buttonStyle(.borderedProminent)
                .keyboardShortcut(.defaultAction)
                .disabled(server.isEmpty)
            Spacer()
        }
        .padding(24)
    }
}

/// Pairing this Mac, in the popover: its person's copies are encrypted with
/// a key it lacks (specs/redesign.md §11).
struct PairPrompt: View {
    @ObservedObject var model: Model
    var body: some View {
        VStack(spacing: 12) {
            Spacer()
            Image(systemName: "lock.shield").font(.system(size: 34)).foregroundStyle(Color.accentColor)
            Text("Pair this Mac").font(.title3.weight(.semibold))
            Text("Your copies are encrypted with a key your devices share. Get a pairing code from one that has it: in its Midgard app, Settings → Encryption, or with mg pair.")
                .font(.callout).foregroundStyle(.secondary).multilineTextAlignment(.center)
            CodeField(model: model)
            Spacer()
        }
        .padding(24)
    }
}

/// Where a pairing code goes in, and why it did not work.
struct CodeField: View {
    @ObservedObject var model: Model
    @State private var code = ""
    var body: some View {
        VStack(spacing: 10) {
            TextField("Pairing code", text: $code, prompt: Text("XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XX"))
                .textFieldStyle(.roundedBorder)
                .font(.system(.body, design: .monospaced))
                .frame(maxWidth: 300)
                .onSubmit { model.pairJoin(code) }
            HStack(spacing: 8) {
                Button("Pair") { model.pairJoin(code) }
                    .buttonStyle(.borderedProminent)
                    .keyboardShortcut(.defaultAction)
                    .disabled(code.isEmpty || model.pairing)
                if model.pairing { ProgressView().controlSize(.small) }
            }
            if let problem = model.pairingProblem {
                Text(problem).font(.caption).foregroundStyle(.red).multilineTextAlignment(.center)
            }
        }
    }
}

/// Signing in, in the popover.
struct SignInPrompt: View {
    @ObservedObject var model: Model
    var body: some View {
        VStack(spacing: 12) {
            Spacer()
            Logo(size: 56)
            Text("Sign in to sync").font(.title3.weight(.semibold))
            if let link = model.signInLink {
                Text("Approve the sign-in in your browser.")
                    .font(.callout).foregroundStyle(.secondary)
                ProgressView().controlSize(.small)
                Button("Open the Link Again") { NSWorkspace.shared.open(link) }.buttonStyle(.link)
            } else {
                Text("Midgard signs you in through \(model.status.server.map { URL(string: $0)?.host ?? $0 } ?? "your server") and auth.latere.ai, in your browser.")
                    .font(.callout).foregroundStyle(.secondary).multilineTextAlignment(.center)
                Button("Sign In…") { model.signIn() }
                    .buttonStyle(.borderedProminent)
                    .keyboardShortcut(.defaultAction)
            }
            if let problem = model.problem, problem != "Sign in to sync." {
                Text(problem).font(.caption).foregroundStyle(.secondary).multilineTextAlignment(.center)
            }
            Spacer()
        }
        .padding(24)
    }
}
