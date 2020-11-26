// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"golang.design/x/midgard/config"
	"golang.design/x/midgard/pkg/term"
	"golang.design/x/midgard/pkg/types"
	"golang.design/x/midgard/pkg/types/proto"
	"golang.design/x/midgard/pkg/utils"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "check midgard setup status",
	Long:  `check midgard setup status`,
	Args:  cobra.ExactArgs(0),
	Run: func(_ *cobra.Command, args []string) {
		var status string

		// check server status
		res, err := utils.Request(http.MethodGet,
			config.Get().Domain+"/midgard/ping", nil)
		if err != nil {
			status += fmt.Sprintf("server status: %s, %v\n", term.Red("request error"), err)
		} else {
			var out types.PingOutput
			err = json.Unmarshal(res, &out)
			if err != nil {
				status += fmt.Sprintf("server status: %s, %v\n", term.Red("parse error"), err)
			} else {
				status += fmt.Sprintf("server status: %s\n", term.Green("OK"))
			}
		}

		// check daemon status
		utils.Connect(func(ctx context.Context, c proto.MidgardClient) {
			_, err := c.Ping(ctx, &proto.PingInput{})
			if err != nil {
				status += fmt.Sprintf("daemon status: %s, %v\n", term.Red("error"), err)
			} else {
				status += fmt.Sprintf("daemon status: %s\n", term.Green("OK"))
			}
		})

		fmt.Println(status)
	},
}
