// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"os"

	"changkun.de/x/midgard/internal/signin"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign this device in to midgard through auth.latere.ai",
	Long: `Sign this device in to midgard through auth.latere.ai.

It prints a link and a code; open the link in any browser, approve, and this
device is signed in: its daemon and mg commands reach your clipboard, and only
yours. The sign-in is kept until mg logout.`,
	Args: cobra.ExactArgs(0),
	Run: func(_ *cobra.Command, _ []string) {
		if err := signin.Login(context.Background()); err != nil {
			errorf("cannot sign in: %v", err)
			os.Exit(1)
		}
		errorf("signed in.")
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign this device out of midgard",
	Args:  cobra.ExactArgs(0),
	Run: func(_ *cobra.Command, _ []string) {
		if err := signin.Logout(); err != nil {
			errorf("cannot sign out: %v", err)
			os.Exit(1)
		}
		errorf("signed out.")
	},
}
