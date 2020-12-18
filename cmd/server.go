// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"changkun.de/x/midgard/api/rest"
	"github.com/spf13/cobra"
)

// serverCmd runs the midgard server.
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run the Midgard server",
	Long:  `Run the Midgard server`,
	Args:  cobra.ExactArgs(0),
	Run: func(_ *cobra.Command, args []string) {
		m := rest.NewMidgard()
		m.Serve()
	},
}
