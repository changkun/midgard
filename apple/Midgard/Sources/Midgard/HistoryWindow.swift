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
                            Label(shortName(d), systemImage: d == model.status.name ? "laptopcomputer" : "desktopcomputer")
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
                        Button("Copy") { model.use(item) }.disabled(item.waiting)
                        Button("Delete", role: .destructive) { model.delete(item) }.disabled(item.waiting)
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

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            content
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
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
                    Button(role: .destructive) { model.delete(item) } label: { Label("Delete", systemImage: "trash") }
                    Spacer()
                }
                .disabled(item.waiting)
            }
            .padding(16)
        }
    }

    @ViewBuilder private var content: some View {
        if let (mime, data) = item.waiting ? nil : Engine.get(item.seq) {
            if mime == "image/png", let image = NSImage(data: data) {
                Image(nsImage: image).resizable().scaledToFit()
                    .padding(20)
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .background(Color(nsColor: .textBackgroundColor))
            } else {
                ScrollView {
                    Text(String(decoding: data, as: UTF8.self))
                        .font(.system(.body, design: .monospaced))
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .topLeading)
                        .padding(20)
                }
                .background(Color(nsColor: .textBackgroundColor))
            }
        } else {
            Text(item.preview ?? "")
                .textSelection(.enabled)
                .padding(20)
        }
    }
}
