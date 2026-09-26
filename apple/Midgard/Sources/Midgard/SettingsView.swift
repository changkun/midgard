// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import AppKit
import SwiftUI

/// Midgard's settings, in ⌘, as a Mac app's are.
struct SettingsView: View {
    @ObservedObject var model: Model
    var body: some View {
        TabView {
            General(model: model).tabItem { Label("General", systemImage: "gearshape") }
            Account(model: model).tabItem { Label("Account", systemImage: "person.crop.circle") }
            About(model: model).tabItem { Label("About", systemImage: "info.circle") }
        }
        .frame(width: 460, height: 320)
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
