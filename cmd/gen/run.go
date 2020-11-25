// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package gen

import (
	"github.com/spf13/cobra"
	"golang.design/x/midgard/pkg/cli"
)

var (
	fpath string
)

// Cmd ...
var Cmd = &cobra.Command{
	Use:   "new",
	Short: "new creates a public accessible url for a specific resource",
	Long:  `new creates a public accessible url for a specific resource`,
	Args:  cobra.MaximumNArgs(1),
	Run:   run,
}

func init() {
	Cmd.PersistentFlags().StringVarP(&fpath, "for", "f", "", "path to a file you want to create its public url")
}

func run(_ *cobra.Command, args []string) {
	uri := ""
	if len(args) > 0 {
		uri = args[0]
	}
	cli.AllocateURLgRPC(uri, fpath)
}
