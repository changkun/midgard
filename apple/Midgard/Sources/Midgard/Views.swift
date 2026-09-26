// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import SwiftUI

/// The menu in the menu bar.
struct MenuView: View {
    @ObservedObject var model: Model
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        Text(model.summary)
        if let link = model.signInLink {
            Button("Approve the sign-in in your browser…") { NSWorkspace.shared.open(link) }
        }
        Divider()
        let recent = model.history.filter { !$0.waiting }.prefix(8)
        if recent.isEmpty {
            Text("No copies yet")
        } else {
            ForEach(Array(recent)) { item in
                Button(label(item)) { model.use(item) }
            }
        }
        Divider()
        Button("Open History…") {
            openWindow(id: "history")
            NSApp.activate(ignoringOtherApps: true)
        }
        Button("Share Clipboard at a Link") { model.shareClipboard() }
            .keyboardShortcut("s", modifiers: [.control, .option])
        if let url = model.lastShare {
            Button("Copy \(url.absoluteString)") { model.pasteboard.write(mime: "text", data: Data(url.absoluteString.utf8)) }
        }
        Divider()
        Toggle("Pause Syncing", isOn: $model.paused)
        Toggle("Start at Login", isOn: Binding(get: { model.startsAtLogin }, set: { model.startsAtLogin = $0 }))
        if model.status.configured {
            if model.status.signedIn {
                Button("Sign Out") { model.signOut() }
            } else {
                Button("Sign In…") { model.signIn() }
            }
        }
        Button("Settings…") {
            openWindow(id: "settings")
            NSApp.activate(ignoringOtherApps: true)
        }
        Divider()
        Button("Quit midgard") { NSApp.terminate(nil) }.keyboardShortcut("q")
    }

    private func label(_ item: HistoryItem) -> String {
        if item.isImage { return "Image, \(ByteCountFormatter.string(fromByteCount: Int64(item.size), countStyle: .file))" }
        let line = (item.preview ?? "").split(whereSeparator: \.isNewline).first.map(String.init) ?? ""
        return line.count > 48 ? String(line.prefix(47)) + "…" : line
    }
}

/// The history window: every copy, from all your devices.
struct HistoryView: View {
    @ObservedObject var model: Model
    @State private var search = ""
    @State private var selection: HistoryItem.ID?

    var body: some View {
        NavigationSplitView {
            List(filtered, selection: $selection) { item in
                Row(item: item).tag(item.id)
                    .contextMenu {
                        Button("Copy") { model.use(item) }.disabled(item.waiting)
                        Button("Delete", role: .destructive) { model.delete(item) }.disabled(item.waiting)
                    }
            }
            .searchable(text: $search, prompt: "Search your history")
            .navigationSplitViewColumnWidth(min: 260, ideal: 320)
            .overlay {
                if model.history.isEmpty {
                    Text(model.status.running ? "No copies yet." : model.summary)
                        .foregroundStyle(.secondary).padding()
                }
            }
        } detail: {
            if let item = model.history.first(where: { $0.id == selection }) {
                Detail(item: item, model: model)
            } else {
                Text("Select a copy").foregroundStyle(.secondary)
            }
        }
        .toolbar {
            ToolbarItem {
                Button("Clear History", role: .destructive) { model.clearHistory() }
                    .disabled(model.history.isEmpty)
            }
        }
        .navigationTitle("midgard")
        .onAppear { model.refresh() }
    }

    private var filtered: [HistoryItem] {
        guard !search.isEmpty else { return model.history }
        return model.history.filter { ($0.preview ?? "").localizedCaseInsensitiveContains(search) }
    }
}

private struct Row: View {
    let item: HistoryItem
    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(item.isImage ? "Image" : (item.preview ?? "").trimmingCharacters(in: .whitespacesAndNewlines))
                .lineLimit(2)
            Text(meta).font(.caption).foregroundStyle(.secondary)
        }
        .padding(.vertical, 2)
    }
    private var meta: String {
        let when = item.date.formatted(date: .abbreviated, time: .shortened)
        let size = ByteCountFormatter.string(fromByteCount: Int64(item.size), countStyle: .file)
        return item.waiting ? "\(when) · waiting to sync · \(size)" : "\(when) · \(item.origin) · \(size)"
    }
}

private struct Detail: View {
    let item: HistoryItem
    @ObservedObject var model: Model

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            ScrollView {
                content.frame(maxWidth: .infinity, alignment: .topLeading).padding()
            }
            HStack {
                Button("Copy") { model.use(item) }.keyboardShortcut("c")
                Button("Delete", role: .destructive) { model.delete(item) }
                Spacer()
                Text("\(item.origin) · \(item.date.formatted(date: .abbreviated, time: .standard))")
                    .font(.caption).foregroundStyle(.secondary)
            }
            .padding([.horizontal, .bottom])
            .disabled(item.waiting)
        }
    }

    @ViewBuilder private var content: some View {
        if let (mime, data) = Engine.get(item.seq) {
            if mime == "image/png", let image = NSImage(data: data) {
                Image(nsImage: image).resizable().scaledToFit()
            } else {
                Text(String(decoding: data, as: UTF8.self)).textSelection(.enabled)
                    .font(.system(.body, design: .monospaced))
            }
        } else {
            Text(item.preview ?? "").textSelection(.enabled)
        }
    }
}

/// Settings: the server, and the sign-in.
struct SettingsView: View {
    @ObservedObject var model: Model
    @State private var server = ""

    var body: some View {
        Form {
            Section {
                TextField("Server", text: $server, prompt: Text("example.com, or http://mg.local:8456"))
                    .onSubmit { model.setServer(server) }
                Button("Save") { model.setServer(server) }.disabled(server.isEmpty)
            } header: {
                Text("Your midgard server")
            } footer: {
                Text("Where your devices meet. It passes copies between them and keeps none.")
                    .font(.caption).foregroundStyle(.secondary)
            }
            Section("Sign-in") {
                if model.status.signedIn {
                    Text("Signed in.")
                    Button("Sign Out") { model.signOut() }
                } else if let link = model.signInLink {
                    Text("Approve the sign-in in your browser.")
                    Button("Open the link again") { NSWorkspace.shared.open(link) }
                } else {
                    Button("Sign In…") { model.signIn() }.disabled(!model.status.configured)
                }
            }
            if let problem = model.problem {
                Section { Text(problem).foregroundStyle(.secondary) }
            }
        }
        .formStyle(.grouped)
        // A grouped form scrolls, and so has no height of its own: a window
        // sized to it had none either, and showed nothing but its title.
        .frame(width: 440, height: 380)
        .onAppear { server = model.status.server ?? "" }
    }
}
