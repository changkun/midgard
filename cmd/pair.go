// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"time"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/device"
	"changkun.de/x/midgard/internal/e2e"
	"github.com/spf13/cobra"
)

// pairCmd gives another device the person's key, or takes it with a code
// (specs/redesign.md §11).
var pairCmd = &cobra.Command{
	Use:   "pair [code]",
	Short: "Give another device your key, or take it with a code",
	Long: `Your copies are encrypted with a key your devices share: the server passes
them, and cannot read them. A device gets the key by pairing with one that
has it.

  mg pair           on a machine that has the key: show a code for another
  mg pair <code>    on a machine that needs it: take the key with the code

A code works once, for ten minutes. On a phone, open the link mg pair shows.
The first of your devices to connect makes the key; you pair the others.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		if len(args) == 1 {
			joinPairing(args[0])
			return
		}
		showPairing()
	},
}

// personKey is the key this machine keeps for its person; nil when it has
// none.
func personKey() *e2e.Key {
	k, _, err := device.LoadKey()
	if err != nil {
		fail(exitFailed, "cannot read this machine's key: %v", err)
	}
	return k
}

// pairing is what mg pair prints with --json.
type pairing struct {
	Code    string    `json:"code"`
	Link    string    `json:"link"`
	Expires time.Time `json:"expires"`
}

// showPairing leaves the key for another device, and shows the code that
// takes it.
func showPairing() {
	k, since, err := device.LoadKey()
	if err != nil {
		fail(exitFailed, "cannot read this machine's key: %v", err)
	}
	if k == nil {
		kid, err := client.Kid()
		exitOn(err, "ask the server about your key")
		if kid == "" {
			fail(exitNotFound, "your devices have no key yet: the first to connect makes it, the Mac app or mg daemon")
		}
		fail(exitNotPaired, "this machine does not have your key: run mg pair <code> here, with a code from a device that has it")
	}
	code, err := device.ShowPairing(k, since)
	exitOn(err, "leave the key for another device")
	p := pairing{Code: code.String(), Link: config.ServerURL() + "/midgard/#" + code.Link(), Expires: time.Now().Add(10 * time.Minute)}
	if jsonOut {
		printJSON(p)
		return
	}
	fmt.Println(p.Code)
	errorf("On the other device, run: mg pair %s", p.Code)
	errorf("On a phone or in a browser, open: %s", p.Link)
	errorf("The code works once, for ten minutes.")
}

// joinPairing takes the key with the code another device showed.
func joinPairing(given string) {
	code, err := e2e.ParseCode(given)
	if err != nil {
		fail(exitUsage, "%q is not a pairing code: it is %d letters and digits, as the other device shows it", given, e2e.CodeLen)
	}
	k, since, err := device.JoinPairing(code)
	if errors.Is(err, client.ErrNotFound) {
		fail(exitNotFound, "no pairing waits for that code: it was used, or ten minutes passed; show a new one")
	}
	exitOn(err, "pair")
	exitOn(device.SaveKey(k, since), "keep the key")
	if jsonOut {
		printJSON(struct {
			Kid string `json:"kid"`
		}{k.ID()})
		return
	}
	errorf("Paired: this machine has your key. mg daemon, if it runs here, uses it within a minute.")
}
