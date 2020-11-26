// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"golang.design/x/midgard/pkg/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of the Midgard",
	Long:  `Print the version number of the Midgard`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.String())
	},
}
