// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"changkun.de/x/midgard/pkg/config"
	"changkun.de/x/midgard/pkg/types/proto"
	"changkun.de/x/midgard/pkg/utils"
	"github.com/spf13/cobra"
)

var code2imgCmd = &cobra.Command{
	Use:   "code2img [codefile]",
	Short: "creates a public url for the code in your clipboard and a url of a rendered image",
	Long: `code2img creates a public url for the code in your clipboard and a url of a rendered image.

If you don't attach any file, then the midgard will try to use the clipboard data directly.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		utils.Connect(func(ctx context.Context, c proto.MidgardClient) {
			codepath := ""
			if len(args) > 0 {
				var err error
				codepath, err = filepath.Abs(args[0])
				if err != nil {
					log.Println("cannot find your file:", err)
					return
				}
			}

			out, err := c.CodeToImage(ctx, &proto.CodeToImageInput{
				CodePath: codepath,
			})
			if err != nil {
				log.Println("cannot convert your code to image:", err)
				return
			}

			log.Println("your code and image urls are ready:")
			fmt.Println(config.Get().Domain + out.CodeURL)
			fmt.Println(config.Get().Domain + out.ImageURL)
			log.Println("and the image url is already in your clipboard for pasting.")
		})
	},
}
