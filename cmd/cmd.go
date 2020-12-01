// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
// interactive = flag.String("i", "", "interactively input content")
)

func usage() {
	fmt.Fprintf(os.Stderr, `usage: midgard [-s] [-d]
options:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `example:
`)
	os.Exit(2)
}

// Execute executes the midgard commands.
func Execute() {
	log.SetPrefix("midgard: ")
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)

	var rootCmd = &cobra.Command{
		Use:   "midgard",
		Short: "midgard is a lightweight solution for managing personal resource namespace.",
		Long: `midgard is a lightweight solution for managing personal resource namespace.
See: https://golang.design/s/midgard for more details.
`,
	}

	rootCmd.AddCommand(
		versionCmd,
		serverCmd,
		daemonCmd,
		allocCmd,
		statusCmd,
		newsCmd,
	)
	rootCmd.Execute()
}
