// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"errors"
	"os"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/signin"
)

// mg is used by people and, as much, by agents and scripts
// (specs/redesign.md §3): what it prints for a person goes to a terminal,
// and with --json what it found is printed as JSON instead, on stdout, while
// messages stay on stderr. Every command ends with one of these codes, so a
// script can tell what went wrong without reading the message.
const (
	exitOK        = 0
	exitFailed    = 1 // the server or the network failed, or refused
	exitUsage     = 2 // the command was used wrongly
	exitSignedIn  = 3 // this device is not signed in: mg login, or a token
	exitOffline   = 4 // none of your devices is online to answer
	exitNotFound  = 5 // no such copy, share, device or token
	exitNotPaired = 6 // this device does not have its person's key: mg pair
)

// jsonOut is --json: print results as JSON.
var jsonOut bool

// printJSON prints v as JSON on stdout.
func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fail(exitFailed, "cannot print the result: %v", err)
	}
}

// fail stops the command with a message on stderr and the exit code.
func fail(code int, format string, args ...any) {
	errorf(format, args...)
	os.Exit(code)
}

// exitOn stops the command when err is not nil, with a message saying what
// it could not do, and the exit code for why.
func exitOn(err error, doing string) {
	if err == nil {
		return
	}
	code := exitCode(err)
	switch code {
	case exitOffline:
		fail(code, "cannot %s: none of your devices is online; your clipboard and history are on them", doing)
	case exitNotFound:
		fail(code, "cannot %s: there is no such thing", doing)
	case exitNotPaired:
		fail(code, "cannot %s: this machine does not have your key; pair it: mg pair <code>, with a code from one of your devices", doing)
	default:
		fail(code, "cannot %s: %v", doing, err)
	}
}

// exitCode is the code a command ends with for err.
func exitCode(err error) int {
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, signin.ErrSignedOut):
		return exitSignedIn
	case errors.Is(err, client.ErrNoDevice):
		return exitOffline
	case errors.Is(err, client.ErrNotFound):
		return exitNotFound
	case errors.Is(err, client.ErrNotPaired):
		return exitNotPaired
	default:
		return exitFailed
	}
}
