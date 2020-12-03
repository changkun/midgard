// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"changkun.de/x/midgard/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of the Midgard",
	Long:  `Print the version number of the Midgard`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.String())
	},
}
