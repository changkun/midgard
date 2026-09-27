// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import CoreImage.CIFilterBuiltins
import SwiftUI

/// Midgard's settings, in ⌘, as a Mac app's are.
struct SettingsView: View {
    @ObservedObject var model: Model
    var body: some View {
        TabView {
            General(model: model).tabItem { Label("General", systemImage: "gearshape") }
            Account(model: model).tabItem { Label("Account", systemImage: "person.crop.circle") }
            Encryption(model: model).tabItem { Label("Encryption", systemImage: "lock.shield") }
            About(model: model).tabItem { Label("About", systemImage: "info.circle") }
        }
        .frame(width: 460, height: 380)
    }
}

private struct General: View {
    @ObservedObject var model: Model
    var body: some View {
        Form {
            Toggle("Start Midgard when you log in", isOn: Binding(get: { model.startsAtLogin }, set: { model.startsAtLogin = $0 }))
            Toggle("Pause syncing this Mac's clipboard", isOn: $model.paused)
            LabeledContent("Share the clipboard at a link") {
                Text("⌃⌥S").font(.system(.body, design: .rounded)).foregroundStyle(.secondary)
            }
            LabeledContent("History") {
                Text("The last 200 copies, for 30 days, on every device").foregroundStyle(.secondary)
            }
        }
        .formStyle(.grouped)
        .onAppear { model.checkLoginItem() }
    }
}

private struct Account: View {
    @ObservedObject var model: Model
    @State private var server = ""
    var body: some View {
        Form {
            Section {
                TextField("Server", text: $server, prompt: Text("example.com, or http://mg.local:8456"))
                    .onSubmit { model.setServer(server) }
                HStack {
                    Spacer()
                    Button("Use This Server") { model.setServer(server) }
                        .disabled(server.isEmpty || server == model.status.server?.replacingOccurrences(of: "https://", with: ""))
                }
            } header: {
                Text("Your Midgard server")
            } footer: {
                Text("Where your devices meet. It passes copies between them and keeps none.")
                    .font(.caption).foregroundStyle(.secondary)
            }
            Section("Sign-in") {
                if model.status.signedIn {
                    LabeledContent("Signed in", value: "through auth.latere.ai")
                    LabeledContent("This Mac", value: shortName(model.status.name ?? "—"))
                    HStack { Spacer(); Button("Sign Out") { model.signOut() } }
                } else if let link = model.signInLink {
                    LabeledContent("Waiting") { ProgressView().controlSize(.small) }
                    Button("Open the sign-in link again") { NSWorkspace.shared.open(link) }.buttonStyle(.link)
                } else {
                    HStack { Spacer(); Button("Sign In…") { model.signIn() }.disabled(!model.status.configured) }
                }
            }
            if let problem = model.problem {
                Section { Text(problem).foregroundStyle(.secondary) }
            }
        }
        .formStyle(.grouped)
        .onAppear {
            server = (model.status.server ?? "").replacingOccurrences(of: "https://", with: "")
        }
    }
}

/// End-to-end encryption (specs/redesign.md §11): whether this Mac has its
/// person's key, and pairing, to give the key to another device, or to take
/// it from one.
private struct Encryption: View {
    @ObservedObject var model: Model
    @State private var shown: (code: String, link: URL)?
    @State private var problem: String?
    @State private var asking = false

    var body: some View {
        Form {
            if model.status.sealing {
                Section {
                    LabeledContent("Encryption", value: "On: this Mac has your key")
                    if let shown {
                        PairingCode(code: shown.code, link: shown.link)
                    } else {
                        HStack {
                            Spacer()
                            if asking { ProgressView().controlSize(.small) }
                            Button("Pair Another Device…") { show() }.disabled(asking)
                        }
                    }
                    if let problem { Text(problem).font(.caption).foregroundStyle(.red) }
                } footer: {
                    Text("Your copies are encrypted with a key your devices share: your server passes them, and cannot read them. A device gets the key by pairing with one that has it.")
                        .font(.caption).foregroundStyle(.secondary)
                }
                Section {
                    Toggle("Let iPhone Shortcuts in, unencrypted", isOn: Binding(get: { model.status.bridge }, set: { model.setBridge($0) }))
                } footer: {
                    Text("Shortcuts cannot encrypt. With this on, this Mac hands your newest copy to your server in the clear, for Get from Midgard, and takes what Send to Midgard sends: your server can read both, and could send copies of its own. The web page on your iPhone's Home Screen needs none of this.")
                        .font(.caption).foregroundStyle(.secondary)
                }
            } else if model.status.needsPairing {
                Section {
                    LabeledContent("Encryption", value: "This Mac needs your key")
                    CodeField(model: model).frame(maxWidth: .infinity)
                } footer: {
                    Text("Get a code from a device that has your key: in its Midgard app, Settings → Encryption, or with mg pair.")
                        .font(.caption).foregroundStyle(.secondary)
                }
            } else {
                Section {
                    LabeledContent("Encryption", value: model.phase == .ready ? "Off: your server does not encrypt yet" : "Not set up yet")
                } footer: {
                    Text("Once signed in, the first of your devices to connect makes your key, and you pair the others with it.")
                        .font(.caption).foregroundStyle(.secondary)
                }
            }
        }
        .formStyle(.grouped)
    }

    private func show() {
        asking = true
        problem = nil
        Task {
            switch await model.pairShow() {
            case .success(let s): shown = s
            case .failure(let e): problem = e.message
            }
            asking = false
        }
    }
}

/// A pairing code, to type, and as a QR of its link, for a phone.
struct PairingCode: View {
    let code: String
    let link: URL
    var body: some View {
        HStack(alignment: .top, spacing: 16) {
            if let qr = qrCode(link.absoluteString) {
                Image(nsImage: qr).interpolation(.none).resizable().frame(width: 132, height: 132)
                    .accessibilityLabel("A QR code of the pairing link")
            }
            VStack(alignment: .leading, spacing: 8) {
                Text(code).font(.system(.body, design: .monospaced).weight(.semibold)).textSelection(.enabled)
                    .lineLimit(1).minimumScaleFactor(0.6)
                Text("On the other device, enter this code in its Midgard app, or after mg pair. On a phone, scan the QR. It works once, for ten minutes.")
                    .font(.caption).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(.vertical, 4)
    }
}

/// A QR code of text, as CoreImage draws it.
func qrCode(_ text: String) -> NSImage? {
    let filter = CIFilter.qrCodeGenerator()
    filter.message = Data(text.utf8)
    filter.correctionLevel = "M"
    guard let image = filter.outputImage?.transformed(by: CGAffineTransform(scaleX: 8, y: 8)) else { return nil }
    let rep = NSCIImageRep(ciImage: image)
    let out = NSImage(size: rep.size)
    out.addRepresentation(rep)
    return out
}

private struct About: View {
    @ObservedObject var model: Model
    var body: some View {
        VStack(spacing: 10) {
            Logo(size: 72)
            Text("Midgard").font(.title2.weight(.semibold))
            Text("Version \(model.version)").font(.callout).foregroundStyle(.secondary)
            Text("Your clipboard, and its history, on all your devices.\nYour server passes copies between them and keeps none.")
                .font(.callout).multilineTextAlignment(.center).foregroundStyle(.secondary)
            Link("github.com/changkun/midgard", destination: URL(string: "https://github.com/changkun/midgard")!)
                .font(.callout)
        }
        .padding(24)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

/// The first start's window: the server, the sign-in, and where Midgard
/// lives then, the menu bar.
struct WelcomeView: View {
    @ObservedObject var model: Model
    let done: () -> Void
    var body: some View {
        Group {
            switch model.phase {
            case .needsServer: Setup(model: model)
            case .needsSignIn: SignInPrompt(model: model)
            case .needsPairing: PairPrompt(model: model)
            case .blocked(let why): Notice(symbol: "exclamationmark.triangle", title: "Midgard cannot start", text: why)
            case .ready: allSet
            }
        }
        .frame(width: 420, height: 400)
    }

    private var allSet: some View {
        VStack(spacing: 14) {
            Spacer()
            Image(systemName: "checkmark.circle.fill").font(.system(size: 48)).foregroundStyle(.green)
            Text("You're all set").font(.title3.weight(.semibold))
            VStack(alignment: .leading, spacing: 8) {
                Label("Midgard lives in the menu bar: your recent copies are a click away.", systemImage: "menubar.arrow.up.rectangle")
                Label("Copy on any of your devices, paste on this Mac.", systemImage: "arrow.triangle.2.circlepath")
                Label("⌃⌥S shares the clipboard at a link.", systemImage: "link")
            }
            .font(.callout)
            .frame(maxWidth: 320, alignment: .leading)
            Button("Done", action: done).buttonStyle(.borderedProminent).keyboardShortcut(.defaultAction)
            Spacer()
        }
        .padding(24)
    }
}
