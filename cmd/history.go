// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/types"
	"github.com/spf13/cobra"
)

// historyCmd shows and edits the person's clipboard history, which is on
// their devices; the server asks one that is online (specs/redesign.md §6).
var historyCmd = &cobra.Command{
	Use:   "history [show|copy|rm <number> | clear]",
	Short: "Show your clipboard history, from all your devices",
	Long: `Show your clipboard history, from all your devices, newest first.

  mg history              list it
  mg history show <n>     print copy number n, as mg paste prints the clipboard
  mg history copy <n>     make copy number n your clipboard again, on every device
  mg history rm <n>       remove copy number n, from every device
  mg history clear        remove all of it

The history is on your devices: one of them must be online. Copies a password
manager marked are never kept.`,
	Args: cobra.MaximumNArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		switch {
		case len(args) == 0:
			listHistory()
		case len(args) == 2 && (args[0] == "show" || args[0] == "copy" || args[0] == "rm"):
			id, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				fail(exitUsage, "a copy is named by its number, as mg history lists it")
			}
			switch args[0] {
			case "show":
				t, data, err := client.HistoryEntry(id)
				exitOn(err, "read that copy")
				printCopy(t, data)
			case "copy":
				copyFromHistory(id)
			default:
				exitOn(client.DeleteHistoryEntry(id), "remove it")
				errorf("removed.")
			}
		case len(args) == 1 && args[0] == "clear":
			exitOn(client.ClearHistory(), "clear the history")
			errorf("your history is empty.")
		default:
			fail(exitUsage, "usage: mg history [show|copy|rm <number> | clear]")
		}
	},
}

func listHistory() {
	entries, err := client.History()
	exitOn(err, "read the history")
	if jsonOut {
		printJSON(types.HistoryOutput{History: entries})
		return
	}
	if len(entries) == 0 {
		errorf("your history is empty.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "number\tcopied\tdevice\ttype\tsize\tpreview")
	for _, e := range entries {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%s\n",
			e.ID, e.Created.Local().Format(time.DateTime), e.Device, e.Type, e.Size, oneLine(e.Preview, 40))
	}
	w.Flush()
}

// oneLine is the first line of s, cut to n characters.
func oneLine(s string, n int) string {
	s, _, cut := strings.Cut(strings.TrimSpace(s), "\n")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	} else if cut {
		return s + " …"
	}
	return s
}

// copyFromHistory makes copy id the clipboard again, on all the person's
// devices: it goes to the server, which relays it to them.
func copyFromHistory(id int64) {
	t, data, err := client.HistoryEntry(id)
	exitOn(err, "read that copy")
	seq, err := client.Copy(t, data)
	exitOn(err, "put it on your clipboard")
	copyHere(t, data)
	if jsonOut {
		printJSON(copied{Seq: seq, Type: t, Size: len(data)})
		return
	}
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
