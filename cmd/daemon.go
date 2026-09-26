// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

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
			m, err := daemon.NewDaemon()
			if err != nil {
				errorf("%v", err)
				os.Exit(1)
			}
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
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "name\tonline\tlast seen\thas up to\tid")
			for _, d := range devices {
				online := "no"
				if d.Online {
					online = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", d.Name, online, d.LastSeen.Local().Format(time.DateTime), d.Acked, d.ID)
			}
			w.Flush()
		default:
			err = fmt.Errorf("%s is not a valid action", args[0])
		}
	},
}
