// swift-tools-version: 5.9

// The midgard Mac app (specs/redesign.md §3): SwiftUI for the menu bar and
// the window, the pasteboard and the hotkey; everything else is the Go sync
// engine, linked as libmidgard.a (../engine). Sparkle updates it. ../build.sh
// builds both, and the app, with Sparkle.framework in it.

import PackageDescription

let build = Context.packageDirectory + "/../build"

let package = Package(
    name: "Midgard",
    platforms: [.macOS(.v14)],
    dependencies: [
        .package(url: "https://github.com/sparkle-project/Sparkle", from: "2.10.0"),
    ],
    targets: [
        .systemLibrary(name: "CMidgard", path: "Sources/CMidgard"),
        .executableTarget(
            name: "Midgard",
            dependencies: ["CMidgard", .product(name: "Sparkle", package: "Sparkle")],
            path: "Sources/Midgard",
            linkerSettings: [
                .unsafeFlags(["-L" + build]),
                // Sparkle.framework, in the app's Contents/Frameworks
                .unsafeFlags(["-Xlinker", "-rpath", "-Xlinker", "@executable_path/../Frameworks"]),
                // what the Go runtime and its libraries need on macOS
                .linkedFramework("CoreFoundation"),
                .linkedFramework("Security"),
                .linkedLibrary("resolv"),
            ]
        ),
        .testTarget(name: "MidgardTests", dependencies: ["Midgard"], path: "Tests/MidgardTests"),
    ]
)
