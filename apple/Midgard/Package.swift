// swift-tools-version: 5.9

// The midgard Mac app (specs/redesign.md §3): SwiftUI for the menu bar and
// the window, the pasteboard and the hotkey; everything else is the Go sync
// engine, linked as libmidgard.a (../engine). ../build.sh builds both, and
// the app.

import PackageDescription

let build = Context.packageDirectory + "/../build"

let package = Package(
    name: "Midgard",
    platforms: [.macOS(.v13)],
    targets: [
        .systemLibrary(name: "CMidgard", path: "Sources/CMidgard"),
        .executableTarget(
            name: "Midgard",
            dependencies: ["CMidgard"],
            path: "Sources/Midgard",
            linkerSettings: [
                .unsafeFlags(["-L" + build]),
                // what the Go runtime and its libraries need on macOS
                .linkedFramework("CoreFoundation"),
                .linkedFramework("Security"),
                .linkedLibrary("resolv"),
            ]
        ),
        .testTarget(name: "MidgardTests", dependencies: ["Midgard"], path: "Tests/MidgardTests"),
    ]
)
