// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/device"
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

		// check encryption (specs/redesign.md §11): whether this machine has
		// its person's key, which seals and opens their copies
		s += encryptionStatus()

		fmt.Println(s)
	},
}

// encryptionStatus is a line on whether this machine seals: it has its
// person's key, it must pair, or no device of theirs has made a key yet.
func encryptionStatus() string {
	k, _, err := device.LoadKey()
	if err != nil {
		return fmt.Sprintf("encryption: %s, %v\n", term.Red("cannot read this machine's key"), err)
	}
	kid, err := client.Kid()
	switch {
	case err != nil:
		return fmt.Sprintf("encryption: %s, %v\n", term.Red("cannot ask the server"), err)
	case kid == "":
		return "encryption: off; the first of your devices to connect makes your key\n"
	case k != nil && k.ID() == kid:
		return fmt.Sprintf("encryption: %s, this machine has your key\n", term.Green("on"))
	default:
		return fmt.Sprintf("encryption: %s; pair it: mg pair <code>, with a code from one of your devices\n",
			term.Red("this machine does not have your key"))
	}
}

// connected reports whether a device of the machine called host is online.
// Devices are told apart by id; the name is the host's, as it is.
func connected(devices []types.Device, host string) bool {
	for _, d := range devices {
		if d.Online && d.Name == host {
			return true
		}
	}
	return false
}
