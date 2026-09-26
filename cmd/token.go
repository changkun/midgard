// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/token"
	"github.com/spf13/cobra"
)

// tokenCmd manages the device tokens. It runs on the server's machine, in the
// directory the server runs in, and edits the token file directly; the
// running server sees the change on its next request.
var tokenCmd = &cobra.Command{
	Use:   "token [add|ls|rm] [device]",
	Short: "Manage the tokens devices use instead of the server password",
	Long: `Manage the tokens devices use instead of the server password.

  mg server token add laptop   issue a token for the device "laptop"
  mg server token ls           list the devices that have a token
  mg server token rm laptop    revoke the token of "laptop"

Run it where the server runs, from the same directory. Put the token that
add prints into the device's config.yml as "token: <token>".`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(_ *cobra.Command, args []string) {
		s := token.Open(config.TokensPath)
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		switch {
		case args[0] == "add" && name != "":
			tok, err := s.Add(name)
			if err != nil {
				errorf("cannot issue a token: %v", err)
				os.Exit(1)
			}
			errorf("the token for %s, shown only this once:", name)
			fmt.Println(tok)
			errorf("add it to %s's config.yml as: token: <the token>", name)
		case args[0] == "ls" && name == "":
			infos, err := s.List()
			if err != nil {
				errorf("cannot read the tokens: %v", err)
				os.Exit(1)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "device\tcreated")
			for _, i := range infos {
				fmt.Fprintf(w, "%s\t%s\n", i.Name, i.Created.Local().Format(time.DateTime))
			}
			w.Flush()
		case args[0] == "rm" && name != "":
			if err := s.Remove(name); err != nil {
				errorf("cannot revoke the token: %v", err)
				os.Exit(1)
			}
			errorf("the token of %s is revoked.", name)
		default:
			errorf("usage: mg server token add|rm <device>, or mg server token ls")
			os.Exit(2)
		}
	},
}

func init() {
	serverCmd.AddCommand(tokenCmd)
}
