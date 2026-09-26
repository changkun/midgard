// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/types"
	"github.com/spf13/cobra"
)

var (
	fpath string
)

// allocCmd allocate new midgard namespace (aka URL)
var allocCmd = &cobra.Command{
	Use:   "alloc",
	Short: "alloc creates a public accessible url for a specific resource",
	Long:  `alloc creates a public accessible url for a specific resource`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		uri := ""
		if len(args) > 0 {
			uri = args[0]
		}
		allocate(uri, fpath)
	},
}

func init() {
	allocCmd.PersistentFlags().StringVarP(&fpath, "for", "f", "", "path to a file you want to create its public url")
}

// allocate publishes srcpath, or the universal clipboard when there is none,
// at dstpath, and puts the resulting link on the local clipboard.
func allocate(dstpath, srcpath string) {
	var (
		data []byte
		name string
	)
	if srcpath != "" {
		b, err := os.ReadFile(srcpath)
		if err != nil {
			errorf("cannot read %s: %v", srcpath, err)
			os.Exit(1)
		}
		if len(b) == 0 {
			// no data means "the clipboard" to the server
			errorf("%s is empty, there is nothing to publish", srcpath)
			os.Exit(1)
		}
		data, name = b, filepath.Base(srcpath)
	}

	url, err := client.Allocate(dstpath, data, name)
	if err != nil {
		errorf("cannot publish: %v", err)
		os.Exit(1)
	}
	fmt.Println(url)
	// Through the server, so the daemons keep it: on X11 and Wayland a copy
	// this command made would go when it exits (see client.Copy).
	if err := client.Copy(types.MIMEPlainText, []byte(url)); err != nil {
		errorf("the link is not on your clipboard: %v", err)
		return
	}
	copyHere(types.MIMEPlainText, []byte(url))
	errorf("the link is on your clipboard.")
}
