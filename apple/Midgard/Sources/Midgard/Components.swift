// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import SwiftUI

/// The app's icon, as a view.
struct Logo: View {
    var size: CGFloat = 28
    var body: some View {
        Image(nsImage: NSApp.applicationIconImage ?? NSImage())
            .resizable()
            .interpolation(.high)
            .frame(width: size, height: size)
    }
}

/// A dot and a word on what Midgard does now.
struct StatusPill: View {
    @ObservedObject var model: Model
    var body: some View {
        let state = model.state
        HStack(spacing: 5) {
            Circle().fill(Color(nsColor: state.color)).frame(width: 7, height: 7)
            Text(state.label).font(.caption.weight(.medium))
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 3)
        .background(Color(nsColor: state.color).opacity(0.14), in: Capsule())
        .help(model.summary)
    }
}

/// How long ago, as people say it: "2 min. ago".
func ago(_ date: Date) -> String {
    let f = RelativeDateTimeFormatter()
    f.unitsStyle = .abbreviated
    return date.timeIntervalSinceNow > -30 ? "just now" : f.localizedString(for: date, relativeTo: Date())
}

/// One copy in a list: what it is, and where and when it came from.
struct ItemRow: View {
    let item: HistoryItem
    @ObservedObject var model: Model
    var lines = 2
    var tile: CGFloat = 34

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            ItemTile(item: item, model: model, size: tile)
            VStack(alignment: .leading, spacing: 3) {
                Text(title)
                    .lineLimit(lines)
                    .truncationMode(.tail)
                    .foregroundStyle(item.isImage ? .secondary : .primary)
                HStack(spacing: 4) {
                    if item.waiting {
                        Image(systemName: "clock.arrow.circlepath")
                        Text("Waiting to sync")
                    } else {
                        Text(ago(item.date))
                        Text("·")
                        Text(item.origin == model.status.name ? "This Mac" : shortName(item.origin))
                    }
                    if model.justCopied == item.id {
                        Text("·")
                        Label("Copied", systemImage: "checkmark").labelStyle(.titleAndIcon)
                            .foregroundStyle(.green)
                    }
                }
                .font(.caption)
                .foregroundStyle(.secondary)
                .lineLimit(1)
            }
            Spacer(minLength: 0)
        }
        .contentShape(Rectangle())
    }

    private var title: String {
        if item.isImage { return "Image · \(ByteCountFormatter.string(fromByteCount: Int64(item.size), countStyle: .file))" }
        let text = (item.preview ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        return text.isEmpty ? "Text" : text
    }
}

/// A copy's picture: an image's thumbnail, or what kind of copy it is.
struct ItemTile: View {
    let item: HistoryItem
    @ObservedObject var model: Model
    var size: CGFloat

    var body: some View {
        ZStack {
            RoundedRectangle(cornerRadius: size * 0.24, style: .continuous)
                .fill(Color.accentColor.opacity(0.12))
            if let image = model.thumbnail(item) {
                Image(nsImage: image).resizable().scaledToFill()
                    .frame(width: size, height: size)
                    .clipShape(RoundedRectangle(cornerRadius: size * 0.24, style: .continuous))
            } else {
                Image(systemName: item.isImage ? "photo" : looksLikeLink ? "link" : "text.alignleft")
                    .font(.system(size: size * 0.42, weight: .medium))
                    .foregroundStyle(Color.accentColor)
            }
        }
        .frame(width: size, height: size)
    }

    private var looksLikeLink: Bool {
        let p = (item.preview ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        return !p.contains(" ") && (p.hasPrefix("https://") || p.hasPrefix("http://"))
    }
}

/// A button that is an icon, with its name as its help.
struct IconButton: View {
    let name: String
    let symbol: String
    let action: () -> Void
    var body: some View {
        Button(action: action) {
            Image(systemName: symbol)
                .font(.system(size: 14, weight: .medium))
                .frame(width: 30, height: 26)
                .contentShape(Rectangle())
        }
        .buttonStyle(.borderless)
        .help(name)
        .accessibilityLabel(name)
    }
}
