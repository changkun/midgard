// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"changkun.de/x/midgard/api/daemon"
	"changkun.de/x/midgard/internal/service"
	"changkun.de/x/midgard/internal/types/proto"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
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
			log.Printf("failed to start daemon, err: %v", err)
			return
		}

		defer func() {
			if err != nil {
				log.Printf("failed to %s, err: %v", args[0], err)
				return
			}
			if args[0] != "ls" {
				log.Printf("%s action is done.", args[0])
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
			m.Serve()
			os.Exit(0) // this closes clipboard NSApplication on darwin
		case "ls":
			daemon.Connect(func(ctx context.Context, c proto.MidgardClient) {
				out, err := c.ListDaemons(ctx, &proto.ListDaemonsInput{})
				if err != nil {
					log.Println("cannot list daemons:", status.Convert(err).Message())
					return
				}
				log.Println("active daemons:")
				fmt.Println("id\tname")
				for i := 0; i < len(out.Id); i++ {
					fmt.Printf("%d\t%v\n", out.Id[i], out.Daemons[i])
				}
			})
		default:
			err = fmt.Errorf("%s is not a valid action", args[0])
		}
	},
}
