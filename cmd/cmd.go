// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

// Execute executes the midgard commands.
func Execute() {
	log.SetPrefix("midgard: ")
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)

	var r = &cobra.Command{
		Use:   "mg",
		Short: "midgard is a universal clipboard service.",
		Long: `midgard is a universal clipboard service developed by Changkun Ou.
See https://changkun.de/s/midgard for more details.
`,
	}

	r.AddCommand(
		versionCmd,
		serverCmd,
		daemonCmd,
		allocCmd,
		statusCmd,
		code2imgCmd,
	)
	r.Execute()
}
