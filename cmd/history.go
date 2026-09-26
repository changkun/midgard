// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"text/tabwriter"
	"time"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/types"
	"github.com/spf13/cobra"
)

// historyCmd shows and edits the clipboard history the server keeps for the
// person signed in, across all their devices.
var historyCmd = &cobra.Command{
	Use:   "history [copy|rm <number> | clear]",
	Short: "Show your clipboard history, from all your devices",
	Long: `Show your clipboard history, from all your devices, newest first.

  mg history              list it
  mg history copy <n>     put copy number n back on this device's clipboard
  mg history rm <n>       remove copy number n
  mg history clear        remove all of it

Copies a password manager marked are never kept.`,
	Args: cobra.MaximumNArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		switch {
		case len(args) == 0:
			listHistory()
		case len(args) == 2 && (args[0] == "copy" || args[0] == "rm"):
			id, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				errorf("a copy is named by its number, as mg history lists it")
				os.Exit(2)
			}
			if args[0] == "copy" {
				copyFromHistory(id)
			} else {
				exitOn(client.DeleteHistoryEntry(id), "remove it")
				errorf("removed.")
			}
		case len(args) == 1 && args[0] == "clear":
			exitOn(client.ClearHistory(), "clear the history")
			errorf("your history is empty.")
		default:
			errorf("usage: mg history [copy|rm <number> | clear]")
			os.Exit(2)
		}
	},
}

func listHistory() {
	entries, err := client.History()
	exitOn(err, "read the history")
	if len(entries) == 0 {
		errorf("your history is empty.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "number\tcopied\tdevice\ttype\tsize")
	for _, e := range entries {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\n",
			e.ID, e.Created.Local().Format(time.DateTime), e.Device, e.Type, e.Size)
	}
	w.Flush()
}

// copyFromHistory makes copy id the clipboard again, on all the person's
// devices: it goes to the server, whose daemons apply it.
func copyFromHistory(id int64) {
	t, data, err := client.HistoryEntry(id)
	exitOn(err, "read that copy")
	exitOn(client.Copy(t, data), "put it on your clipboard")
	copyHere(t, data)
	errorf("copy %d is on your clipboard.", id)
}

// copyHere also writes data to this device's clipboard, where that outlives
// the command: macOS and Windows keep the clipboard themselves, so it is
// there even without a daemon. On X11 and Wayland the program that copies
// owns what it copied, and a command writing it would take it from the
// daemon that just applied it, then lose it on exit; the daemon keeps it.
func copyHere(t types.MIME, data []byte) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		clipboard.Local.Write(t, data)
	}
}

// exitOn stops the command with a message when err is not nil.
func exitOn(err error, doing string) {
	switch {
	case err == nil:
		return
	case errors.Is(err, client.ErrNotFound):
		errorf("cannot %s: there is no such copy in your history", doing)
	default:
		errorf("cannot %s: %v", doing, err)
	}
	os.Exit(1)
}
