// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"golang.design/x/midgard/config"
	"golang.design/x/midgard/pkg/clipboard"
	"golang.design/x/midgard/pkg/types/proto"
	"golang.design/x/midgard/pkg/utils"
)

var (
	fpath string
)

// newCmd allocate new midgard namespace (aka URL)
var newCmd = &cobra.Command{
	Use:   "new",
	Short: "new creates a public accessible url for a specific resource",
	Long:  `new creates a public accessible url for a specific resource`,
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
	newCmd.PersistentFlags().StringVarP(&fpath, "for", "f", "", "path to a file you want to create its public url")
}

// allocate request the midgard daemon to allocate a given URL for
// a given resource, or the content from the midgard universal clipboard.
func allocate(dstpath, srcpath string) {
	utils.Connect(func(ctx context.Context, c proto.MidgardClient) {
		out, err := c.AllocateURL(ctx, &proto.AllocateURLInput{
			DesiredPath: dstpath,
			SourcePath:  srcpath,
		})
		if err != nil {
			log.Fatalf("cannot interact with midgard daemon, err:\n%v", err)
		}
		if out.URL != "" {
			url := config.Get().Domain + out.URL
			clipboard.Write(utils.StringToBytes(url))
			fmt.Println(url)
		} else {
			fmt.Printf("%v\n", out.Message)
		}
	})
}
