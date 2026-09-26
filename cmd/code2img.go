// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"changkun.de/x/midgard/api/daemon"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types/proto"
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
		// Read the code here and send it; the daemon does not open paths
		// for its callers. With no file, the server renders the clipboard.
		var code string
		if len(args) > 0 {
			var err error
			code, err = readCode(args[0], lineno)
			if err != nil {
				errorf("%v", err)
				return
			}
		}

		daemon.Connect(func(ctx context.Context, c proto.MidgardClient) {
			out, err := c.CodeToImage(ctx, &proto.CodeToImageInput{Code: code})
			if err != nil {
				errorf("cannot convert your code to an image: %v",
					status.Convert(err).Message())
				return
			}

			if len(out.CodeURL) == 0 && len(out.ImageURL) == 0 {
				errorf("nothing was converted to an image.")
				return
			}

			errorf("your code and image urls are ready:")
			fmt.Println(config.Get().Domain + out.CodeURL)
			fmt.Println(config.Get().Domain + out.ImageURL)
			errorf("the image url is ready for pasting.")
		})
	},
}

// readCode returns the code in the file at path, or only the lines given as
// "start:end" (both inclusive, counted from 1) when lines is not empty.
func readCode(path, lines string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read your file: %w", err)
	}
	if lines == "" {
		return string(b), nil
	}

	from, to, ok := strings.Cut(lines, ":")
	if !ok {
		return "", errors.New("invalid line number format, e.g. 10:20")
	}
	start, err1 := strconv.Atoi(from)
	end, err2 := strconv.Atoi(to)
	if err1 != nil || err2 != nil || start < 1 || end < start {
		return "", fmt.Errorf("invalid line numbers %q, e.g. 10:20", lines)
	}

	all := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if start > len(all) {
		return "", fmt.Errorf("%s has only %d lines", path, len(all))
	}
	end = min(end, len(all))
	return strings.Join(all[start-1:end], "\n"), nil
}
