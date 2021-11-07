// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"log"

	"changkun.de/x/midgard/api/daemon"
	"changkun.de/x/midgard/internal/types/proto"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
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

// allocate request the midgard daemon to allocate a given URL for
// a given resource, or the content from the midgard universal clipboard.
func allocate(dstpath, srcpath string) {
	daemon.Connect(func(ctx context.Context, c proto.MidgardClient) {
		out, err := c.AllocateURL(ctx, &proto.AllocateURLInput{
			DesiredPath: dstpath,
			SourcePath:  srcpath,
		})
		if err != nil {
			log.Fatalf("cannot interact with midgard daemon, err:\n%v",
				status.Convert(err).Message())
		}
		if out.URL != "" {
			// Clipboard is updated on the daemon side, we don't have to
			// write clipboard in the allocate command again, see PR#16.
			fmt.Println(out.URL)
		} else {
			fmt.Printf("%v\n", out.Message)
		}
	})
}
