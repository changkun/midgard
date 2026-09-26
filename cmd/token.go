// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/store"
	"github.com/spf13/cobra"
)

var tokenOwner string

// tokenCmd manages app tokens, for the clients that cannot sign in on their
// own, such as an iOS Shortcut. It runs on the server's machine, in the
// directory the server runs in, against the server's database; the running
// server sees a change on its next request.
var tokenCmd = &cobra.Command{
	Use:   "token [add|ls|rm] [name] --owner <owner>",
	Short: "Manage app tokens for clients that cannot sign in",
	Long: `Manage app tokens for clients that cannot sign in, such as an iOS Shortcut.

  mg server token add phone --owner <owner>   issue a token named "phone"
  mg server token ls --owner <owner>          list the owner's tokens
  mg server token rm phone --owner <owner>    revoke it

A token acts for its owner only, and reaches nothing of anyone else's. Run it
where the server runs, from the same directory, and send the token that add
prints as "Authorization: Bearer <token>".`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(_ *cobra.Command, args []string) {
		if tokenOwner == "" {
			errorf("--owner is required: a token acts for one person")
			os.Exit(2)
		}
		name := ""
		if len(args) == 2 {
			name = args[1]
		}

		s, err := store.Open(config.DBPath)
		if err != nil {
			errorf("cannot open the database: %v", err)
			os.Exit(1)
		}
		defer s.Close()
		ctx := context.Background()

		switch {
		case args[0] == "add" && name != "":
			tok, err := s.IssueAppToken(ctx, tokenOwner, name)
			if err != nil {
				errorf("cannot issue a token: %v", err)
				os.Exit(1)
			}
			errorf("the token %s for %s, shown only this once:", name, tokenOwner)
			fmt.Println(tok)
		case args[0] == "ls" && name == "":
			tokens, err := s.AppTokens(ctx, tokenOwner)
			if err != nil {
				errorf("cannot read the tokens: %v", err)
				os.Exit(1)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "name\tcreated")
			for _, t := range tokens {
				fmt.Fprintf(w, "%s\t%s\n", t.Name, t.Created.Local().Format(time.DateTime))
			}
			w.Flush()
		case args[0] == "rm" && name != "":
			err := s.RevokeAppToken(ctx, tokenOwner, name)
			if errors.Is(err, store.ErrNotFound) {
				errorf("%s has no token named %s", tokenOwner, name)
				os.Exit(1)
			}
			if err != nil {
				errorf("cannot revoke the token: %v", err)
				os.Exit(1)
			}
			errorf("the token %s of %s is revoked.", name, tokenOwner)
		default:
			errorf("usage: mg server token add|rm <name> --owner <owner>, or mg server token ls --owner <owner>")
			os.Exit(2)
		}
	},
}

func init() {
	tokenCmd.Flags().StringVar(&tokenOwner, "owner", "", "whose token it is: the principal it acts for")
	serverCmd.AddCommand(tokenCmd)
}
