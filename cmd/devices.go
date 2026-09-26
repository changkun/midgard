// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/types"
	"github.com/spf13/cobra"
)

// devicesCmd lists one's devices, and forgets one. The server holds each
// copy until every device has it (specs/redesign.md §6); a device that is
// gone for good would make it hold them until they expire.
var devicesCmd = &cobra.Command{
	Use:   "devices [forget <id>]",
	Short: "List your devices, or forget one you no longer use",
	Args:  cobra.MaximumNArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		switch {
		case len(args) == 0:
			devices, err := client.Devices()
			exitOn(err, "list your devices")
			if jsonOut {
				printJSON(types.DevicesOutput{Devices: devices})
				return
			}
			printDevices(devices)
		case len(args) == 2 && args[0] == "forget":
			err := client.ForgetDevice(args[1])
			if errors.Is(err, client.ErrNotFound) {
				fail(exitNotFound, "you have no device %s", args[1])
			}
			exitOn(err, "forget it")
			errorf("forgotten; the server no longer holds copies for it, until it connects again.")
		default:
			fail(exitUsage, "usage: mg devices [forget <id>]")
		}
	},
}

func printDevices(devices []types.Device) {
	if len(devices) == 0 {
		errorf("you have no devices yet; run mg daemon on one.")
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
}

// queueCmd shows what the server holds until each device has it, and takes
// back a copy that has not reached any.
var queueCmd = &cobra.Command{
	Use:   "queue [rm <number>]",
	Short: "Show what is on its way to your devices, or take a copy back",
	Args:  cobra.MaximumNArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		switch {
		case len(args) == 0:
			q, err := client.Queue()
			exitOn(err, "read the queue")
			if jsonOut {
				printJSON(q)
				return
			}
			if len(q.Queue) == 0 {
				errorf("nothing is on its way; every device has everything.")
				return
			}
			names := map[string]string{}
			for _, d := range q.Devices {
				names[d.ID] = d.Name
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "number\tcopied\tfrom\ttype\tsize\twaiting for")
			for _, e := range q.Queue {
				var waiting []string
				for _, id := range e.Waiting {
					waiting = append(waiting, names[id])
				}
				what := string(e.Type)
				if e.Kind != "copy" {
					what = e.Kind
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%s\n", e.Seq, e.Created.Local().Format(time.DateTime),
					e.Origin, what, e.Size, strings.Join(waiting, ", "))
			}
			w.Flush()
		case len(args) == 2 && args[0] == "rm":
			seq, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				fail(exitUsage, "%s is not a number from mg queue", args[1])
			}
			err = client.TakeBack(seq)
			if errors.Is(err, client.ErrNotFound) {
				fail(exitNotFound, "the queue holds no copy %d", seq)
			}
			exitOn(err, "take it back")
			errorf("taken back; no device will get it.")
		default:
			fail(exitUsage, "usage: mg queue [rm <number>]")
		}
	},
}
