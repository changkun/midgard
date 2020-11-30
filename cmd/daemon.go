// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"golang.design/x/midgard/api/rpc"
	"golang.design/x/midgard/pkg/service"
)

// daemonCmd runs the midgard's daemon process.
var daemonCmd = &cobra.Command{
	Use:   "daemon [install|uninstall|start|stop|run]",
	Short: "Run the Midgard daemon",
	Long:  `Run the Midgard daemon`,
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		s, err := service.NewService(
			"midgard-daemon",
			"midgard daemon",
			"the Midgard daemon process",
			[]string{"daemon", "run"},
		)
		if err != nil {
			log.Printf("failed to start daemon, err: %v", err)
			return
		}

		defer func() {
			if err != nil {
				log.Printf("failed to %s, err: %v", args[0], err)
				return
			}
			log.Printf("%s action is done.", args[0])
		}()
		switch args[0] {
		case "install":
			err = s.Install()
		case "uninstall":
			err = s.Remove()
		case "start":
			err = s.Start()
		case "stop":
			err = s.Stop()
		case "run":
			m := rpc.NewMidgard()
			m.Serve()
		default:
			err = fmt.Errorf("%s is not a valid action", args[0])
		}
	},
}
