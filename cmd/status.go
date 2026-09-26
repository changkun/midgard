// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/term"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "check midgard setup status",
	Long:  `check midgard setup status`,
	Args:  cobra.ExactArgs(0),
	Run: func(_ *cobra.Command, args []string) {
		var s string

		// check server status
		res, err := utils.Request(http.MethodGet,
			config.ServerURL()+"/midgard/ping", nil)
		if err != nil {
			s += fmt.Sprintf("server status: %s, %v\n",
				term.Red("request error"), err)
		} else {
			var out types.PingOutput
			err = json.Unmarshal(res, &out)
			if err != nil {
				s += fmt.Sprintf("server status: %s, details:\n%v\n",
					term.Red("failed to parse ping response from server"),
					err)
			} else {
				s += fmt.Sprintf("server status: %s\n", term.Green("OK"))
			}
		}

		// check daemon status: the server knows which daemons are
		// connected, and this machine's is named after its host.
		devices, err := client.Devices()
		host, _ := os.Hostname()
		switch {
		case err != nil:
			s += fmt.Sprintf("daemon status: %s, %v\n", term.Red("cannot ask the server"), err)
		case connected(devices, host):
			s += fmt.Sprintf("daemon status: %s\n", term.Green("OK"))
		default:
			s += fmt.Sprintf("daemon status: %s; start it with mg daemon start\n",
				term.Red("not connected"))
		}

		fmt.Println(s)
	},
}

// connected reports whether the daemon of the machine called host is among
// devices. The server adds a suffix when two machines share a name.
func connected(devices []types.Device, host string) bool {
	for _, d := range devices {
		if d.Name == host || strings.HasPrefix(d.Name, host+"-") {
			return true
		}
	}
	return false
}
