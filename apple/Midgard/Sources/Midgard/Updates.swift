// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

import Foundation
import Sparkle

/// Keeps the app up to date, through Sparkle: it reads the appcast of the
/// newest release (SUFeedURL in Info.plist), checks once a day, and asks
/// before it installs an update, which it takes only when signed with the
/// key whose public half is SUPublicEDKey, and with the app's Developer ID.
@MainActor
final class Updates: NSObject, SPUUpdaterDelegate {
    private var controller: SPUStandardUpdaterController!

    override init() {
        super.init()
        controller = SPUStandardUpdaterController(startingUpdater: true, updaterDelegate: self, userDriverDelegate: nil)
        // for a test of the update itself: check at once, without asking
        if ProcessInfo.processInfo.environment["MIDGARD_UPDATE_NOW"] != nil {
            controller.updater.checkForUpdatesInBackground()
        }
    }

    /// Asks the feed now, and says what it found.
    func check() { controller.checkForUpdates(nil) }

    var checksAutomatically: Bool {
        get { controller.updater.automaticallyChecksForUpdates }
        set { controller.updater.automaticallyChecksForUpdates = newValue }
    }

    /// The feed Info.plist names, or, for a test of the update itself,
    /// MIDGARD_UPDATE_FEED.
    nonisolated func feedURLString(for _: SPUUpdater) -> String? {
        ProcessInfo.processInfo.environment["MIDGARD_UPDATE_FEED"]
    }
}
