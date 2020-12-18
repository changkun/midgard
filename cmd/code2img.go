// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types/proto"
	"changkun.de/x/midgard/internal/utils"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
)

var (
	lineno string
)

func init() {
	code2imgCmd.PersistentFlags().StringVarP(&lineno, "lines", "l", "", "line number, start:end")
}

var code2imgCmd = &cobra.Command{
	Use:   "code2img [codefile] [-l start:end]",
	Short: "creates an image version of the code in a file or clipboard",
	Long:  `creates an image version of the code in a file or clipboard.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		utils.Connect(func(ctx context.Context, c proto.MidgardClient) {
			var (
				codepath string
				start    int64
				end      int64
			)
			if len(args) > 0 {
				var err error
				codepath, err = filepath.Abs(args[0])
				if err != nil {
					log.Println("cannot find your file:", err)
					return
				}
				if lineno != "" {
					nos := strings.Split(lineno, ":")
					if len(nos) != 2 {
						log.Println("invalid line number format, e.g. 10:20")
						return
					}
					start, err = strconv.ParseInt(nos[0], 10, 64)
					if err != nil {
						log.Println("invalid line number")
						return
					}
					end, err = strconv.ParseInt(nos[1], 10, 64)
					if err != nil {
						log.Println("invalid line number")
						return
					}
				}
			}

			out, err := c.CodeToImage(ctx, &proto.CodeToImageInput{
				CodePath: codepath,
				Start:    start,
				End:      end,
			})
			if err != nil {
				log.Println("cannot convert your code to image:",
					status.Convert(err).Message())
				return
			}

			if len(out.CodeURL) == 0 && len(out.ImageURL) == 0 {
				log.Println("nothing was convereted to image.")
				return
			}

			log.Println("your code and image urls are ready:")
			fmt.Println(config.Get().Domain + out.CodeURL)
			fmt.Println(config.Get().Domain + out.ImageURL)
			log.Println("and the image url is already for pasting.")
		})
	},
}
