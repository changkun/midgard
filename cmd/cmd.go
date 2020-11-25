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
	"golang.design/x/midgard/cmd/daemon"
	"golang.design/x/midgard/cmd/gen"
	"golang.design/x/midgard/cmd/server"
	"golang.design/x/midgard/cmd/version"
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

	var cmdVersion = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of the Midgard",
		Long:  `Print the version number of the Midgard`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.String())
		},
	}

	var cmdServer = &cobra.Command{
		Use:   "server",
		Short: "Run the Midgard server",
		Long:  `Run the Midgard server`,
		Args:  cobra.ExactArgs(0),
		Run:   server.Run,
	}

	var cmdDaemon = &cobra.Command{
		Use:   "daemon",
		Short: "Run the Midgard daemon",
		Long:  `Run the Midgard daemon`,
		Args:  cobra.ExactArgs(0),
		Run:   daemon.Run,
	}

	var rootCmd = &cobra.Command{
		Use:   "midgard",
		Short: "midgard is a lightweight solution for managing personal resource namespace.",
		Long: `midgard is a lightweight solution for managing personal resource namespace.
See: https://golang.design/s/midgard for more details.
`,
	}

	rootCmd.AddCommand(
		cmdVersion,
		cmdServer,
		cmdDaemon,
		gen.Cmd,
	)
	rootCmd.Execute()
}
