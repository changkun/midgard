// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/types"
	"github.com/spf13/cobra"
)

var (
	fpath   string
	expires time.Duration
)

// shareCmd publishes a file, or the clipboard, at a link.
var shareCmd = &cobra.Command{
	Use:     "share [name] [-f file] [--expires 24h]",
	Aliases: []string{"alloc"},
	Short:   "Share a file, or your clipboard, at a link",
	Long: `Share a file, or your clipboard, at a link anyone can open.

  mg share                        share your clipboard at a random link
  mg share -f report.pdf          share a file
  mg share notes/today -f a.txt   and give its link a name: /midgard/notes/today.txt
  mg share --expires 24h          retire it after a day

The link is put on your clipboard. mg shares lists what you have shared, and
mg shares rm revokes a share.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		share(name, fpath)
	},
}

func init() {
	shareCmd.Flags().StringVarP(&fpath, "for", "f", "", "the file to share, instead of your clipboard")
	shareCmd.Flags().DurationVar(&expires, "expires", 0, "retire the share after this long, e.g. 24h")
}

// share publishes srcpath, or the clipboard when there is none, and puts the
// link on the clipboard.
func share(name, srcpath string) {
	var data []byte
	filename := ""
	if srcpath != "" {
		b, err := os.ReadFile(srcpath)
		if err != nil {
			errorf("cannot read %s: %v", srcpath, err)
			os.Exit(1)
		}
		if len(b) == 0 {
			// no data means "the clipboard" to the server
			errorf("%s is empty, there is nothing to share", srcpath)
			os.Exit(1)
		}
		data, filename = b, filepath.Base(srcpath)
	}

	sh, err := client.Share(name, data, filename, expires)
	if err != nil {
		errorf("cannot share: %v", err)
		os.Exit(1)
	}
	fmt.Println(sh.URL)
	// Through the server, so the daemons keep it: on X11 and Wayland a copy
	// this command made would go when it exits (see client.Copy).
	if err := client.Copy(types.MIMEPlainText, []byte(sh.URL)); err != nil {
		errorf("the link is not on your clipboard: %v", err)
		return
	}
	copyHere(types.MIMEPlainText, []byte(sh.URL))
	errorf("the link is on your clipboard.")
}

// sharesCmd lists and revokes one's shares.
var sharesCmd = &cobra.Command{
	Use:   "shares [rm <id>]",
	Short: "List what you have shared, or revoke a share",
	Args:  cobra.MaximumNArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		switch {
		case len(args) == 0:
			shares, err := client.Shares()
			exitOn(err, "list your shares")
			if len(shares) == 0 {
				errorf("you have shared nothing.")
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "id\tshared\texpires\ttype\tsize\tlink")
			for _, sh := range shares {
				exp := "never"
				if sh.Expires != nil {
					exp = sh.Expires.Local().Format(time.DateTime)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
					sh.Slug, sh.Created.Local().Format(time.DateTime), exp, sh.Type, sh.Size, sh.URL)
			}
			w.Flush()
		case len(args) == 2 && args[0] == "rm":
			err := client.DeleteShare(args[1])
			if errors.Is(err, client.ErrNotFound) {
				errorf("you have no share %s", args[1])
				os.Exit(1)
			}
			exitOn(err, "revoke it")
			errorf("revoked; its links no longer work.")
		default:
			errorf("usage: mg shares [rm <id>]")
			os.Exit(2)
		}
	},
}
