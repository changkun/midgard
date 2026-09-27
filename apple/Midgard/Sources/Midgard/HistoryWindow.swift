// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import SwiftUI

/// The history window: every copy, from all your devices.
struct HistoryWindow: View {
    @ObservedObject var model: Model
    @State private var filter: Filter = .all
    @State private var search = ""
    @State private var selection: HistoryItem.ID?
    @State private var confirmClear = false

    enum Filter: Hashable {
        case all, text, images
        case device(String)
    }

    var body: some View {
        NavigationSplitView {
            List(selection: $filter) {
                Section("Library") {
                    Label("All Copies", systemImage: "tray.full").tag(Filter.all)
                    Label("Text", systemImage: "text.alignleft").tag(Filter.text)
                    Label("Images", systemImage: "photo").tag(Filter.images)
                }
                if !model.devices.isEmpty {
                    Section("From") {
                        ForEach(model.devices, id: \.self) { d in
                            Label(shortName(d), systemImage: deviceSymbol(d, this: d == model.status.name))
                                .tag(Filter.device(d))
                        }
                    }
                }
            }
            .navigationSplitViewColumnWidth(min: 170, ideal: 190)
        } content: {
            List(items, selection: $selection) { item in
                ItemRow(item: item, model: model, lines: 3, tile: 40)
                    .padding(.vertical, 4)
                    .tag(item.id)
                    .contextMenu {
                        Button("Copy") { model.use(item) }
                        Button(item.waiting ? "Take Back" : "Delete", role: .destructive) { model.delete(item) }
                    }
            }
            .overlay {
                if items.isEmpty {
                    Notice(symbol: search.isEmpty ? "doc.on.clipboard" : "magnifyingglass",
                           title: search.isEmpty ? "No copies here" : "No copy matches",
                           text: model.phase == .ready ? "What you copy on any device shows up here." : model.summary)
                }
            }
            .navigationSplitViewColumnWidth(min: 280, ideal: 340)
        } detail: {
            if let item = model.history.first(where: { $0.id == selection }) {
                Detail(item: item, model: model)
            } else {
                Notice(symbol: "hand.point.left", title: "Select a copy", text: "See it whole, copy it back, or delete it.")
            }
        }
        .searchable(text: $search, placement: .toolbar, prompt: "Search your history")
        .toolbar {
            ToolbarItem(placement: .status) { StatusPill(model: model) }
            ToolbarItem {
                Button { confirmClear = true } label: { Label("Clear History", systemImage: "trash") }
                    .disabled(model.history.isEmpty)
                    .help("Clear the history, on every device")
            }
        }
        .confirmationDialog("Clear your history on every device?", isPresented: $confirmClear) {
            Button("Clear History", role: .destructive) { model.clearHistory() }
        } message: {
            Text("Every copy goes, on all your devices. What is on your clipboards now stays.")
        }
        .navigationTitle("Midgard")
        .frame(minWidth: 820, minHeight: 480)
        .onAppear { model.refresh() }
    }

    private var items: [HistoryItem] {
        model.history.filter { item in
            let kind: Bool
            switch filter {
            case .all: kind = true
            case .text: kind = !item.isImage
            case .images: kind = item.isImage
            case .device(let d): kind = item.origin == d
            }
            return kind && (search.isEmpty || (item.preview ?? "").localizedCaseInsensitiveContains(search))
        }
    }
}

/// One copy, whole.
private struct Detail: View {
    let item: HistoryItem
    @ObservedObject var model: Model
    @State private var shown: Shown?

    /// A copy as the pane shows it, made once when it is selected, not each
    /// time the pane draws.
    enum Shown {
        case image(NSImage)
        case text(String, cut: Bool)
    }

    /// The most of a text the pane shows: a Text of more takes seconds to lay
    /// out. Copy copies all of it.
    private static let most = 100_000

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            content
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                .task(id: item.id) { shown = await load() }
            Divider()
            VStack(alignment: .leading, spacing: 10) {
                Grid(alignment: .leading, horizontalSpacing: 14, verticalSpacing: 4) {
                    GridRow { Text("Copied").foregroundStyle(.secondary); Text(item.date.formatted(date: .abbreviated, time: .standard)) }
                    GridRow { Text("From").foregroundStyle(.secondary); Text(item.waiting ? "this Mac, waiting to sync" : shortName(item.origin)) }
                    GridRow { Text("Type").foregroundStyle(.secondary); Text(item.isImage ? "Image (PNG)" : "Text") }
                    GridRow { Text("Size").foregroundStyle(.secondary); Text(ByteCountFormatter.string(fromByteCount: Int64(item.size), countStyle: .file)) }
                }
                .font(.callout)
                HStack {
                    Button { model.use(item) } label: {
                        Label(model.justCopied == item.id ? "Copied" : "Copy", systemImage: model.justCopied == item.id ? "checkmark" : "doc.on.doc")
                    }
                    .keyboardShortcut("c")
                    .buttonStyle(.borderedProminent)
                    Button(role: .destructive) { model.delete(item) } label: {
                        Label(item.waiting ? "Take Back" : "Delete", systemImage: item.waiting ? "arrow.uturn.backward" : "trash")
                    }
                    .help(item.waiting ? "It has not reached your server: take it back, and it goes nowhere" : "Delete it on every device")
                    Spacer()
                }
            }
            .padding(16)
        }
    }

    @ViewBuilder private var content: some View {
        switch shown {
        case .image(let image):
            Image(nsImage: image).resizable().scaledToFit()
                .padding(20)
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(Color(nsColor: .textBackgroundColor))
        case .text(let text, let cut):
            ScrollView {
                VStack(alignment: .leading, spacing: 12) {
                    Text(text)
                        .font(.system(.body, design: .monospaced))
                        .textSelection(.enabled)
                    if cut {
                        Text("The first \(Self.most.formatted()) characters. Copy copies all of it.")
                            .font(.callout).foregroundStyle(.secondary)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .topLeading)
                .padding(20)
            }
            .background(Color(nsColor: .textBackgroundColor))
        case nil:
            Text(item.preview ?? "")
                .textSelection(.enabled)
                .padding(20)
        }
    }

    /// Fetches the copy, and cuts a text, away from the main thread, and
    /// makes it what the pane shows.
    private func load() async -> Shown? {
        let item = item, most = Self.most
        enum Got { case image(Data), text(String, cut: Bool) }
        let got = await Task.detached(priority: .userInitiated) { () -> Got? in
            guard let (mime, data) = Engine.bytes(of: item) else { return nil }
            if mime == "image/png" { return .image(data) }
            let text = String(decoding: data, as: UTF8.self)
            if data.count <= most { return .text(text, cut: false) } // no more characters than bytes
            return text.count > most ? .text(String(text.prefix(most)), cut: true) : .text(text, cut: false)
        }.value
        switch got {
        case .image(let data): return NSImage(data: data).map { .image($0) }
        case .text(let text, let cut): return .text(text, cut: cut)
        case nil: return nil
        }
    }
}
