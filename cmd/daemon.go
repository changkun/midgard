// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"os"

	"changkun.de/x/midgard/api/daemon"
	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/service"
	"github.com/spf13/cobra"
)

// daemonCmd runs the midgard's daemon process.
var daemonCmd = &cobra.Command{
	Use:   "daemon [install|uninstall|start|stop|run|ls]",
	Short: "Interact with the midgard daemon(s)",
	Long:  `Interact with the midgard daemon(s)`,
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		s, err := service.NewService(
			"midgard-daemon",
			"midgard daemon",
			"the Midgard daemon process",
			[]string{"daemon", "run"},
		)
		if err != nil {
			errorf("cannot start the daemon: %v", err)
			return
		}

		defer func() {
			if err != nil {
				errorf("cannot %s: %v", args[0], err)
				return
			}
			if args[0] != "ls" {
				errorf("the %s action is done.", args[0])
			}
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
			m := daemon.NewDaemon()
			onStart, onStop := m.Run(context.Background())

			err = s.Run(onStart, onStop)
			if err == nil {
				os.Exit(0) // this closes clipboard NSApplication on darwin
			}
		case "ls":
			devices, err := client.Devices()
			if err != nil {
				errorf("cannot list the daemons: %v", err)
				return
			}
			errorf("active daemons:")
			fmt.Println("id\tname")
			for _, d := range devices {
				fmt.Printf("%d\t%s\n", d.Index, d.Name)
			}
		default:
			err = fmt.Errorf("%s is not a valid action", args[0])
		}
	},
}
