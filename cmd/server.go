// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"
	"golang.design/x/midgard/api/rest"
)

// serverCmd runs the midgard server.
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run the Midgard server",
	Long:  `Run the Midgard server`,
	Args:  cobra.ExactArgs(0),
	Run: func(*cobra.Command, []string) {
		m := rest.NewMidgard()
		m.Serve()
	},
}
