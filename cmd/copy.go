// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/wire"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// copyCmd copies to the person's devices, from its arguments or stdin: what
// an agent or a script uses to hand you something (specs/redesign.md §3).
var copyCmd = &cobra.Command{
	Use:   "copy [text...]",
	Short: "Copy text, or what comes on stdin, to your devices",
	Long: `Copy text, or what comes on stdin, to the clipboard of all your devices.

  mg copy hello world           the words, joined by spaces
  git log -1 | mg copy          what comes on stdin, as it is
  mg copy < screenshot.png      a PNG image

It reaches devices that are off when they come back.`,
	Run: func(_ *cobra.Command, args []string) {
		var data []byte
		if len(args) > 0 {
			data = []byte(strings.Join(args, " "))
		} else {
			if isTerminal(os.Stdin) {
				fail(exitUsage, "nothing to copy: give text, or pipe it in, e.g. echo hi | mg copy")
			}
			b, err := io.ReadAll(io.LimitReader(os.Stdin, wire.MaxPayload+1))
			if err != nil {
				fail(exitFailed, "cannot read stdin: %v", err)
			}
			data = b
		}
		if len(data) == 0 {
			fail(exitUsage, "nothing to copy")
		}
		if len(data) > wire.MaxPayload {
			fail(exitUsage, "a copy holds at most %d MB", wire.MaxPayload>>20)
		}
		t := types.MIMEPlainText
		switch {
		case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
			t = types.MIMEImagePNG
		case !utf8.Valid(data):
			fail(exitUsage, "only text and PNG images can be copied")
		}
		seq, err := client.Copy(personKey(), t, data)
		exitOn(err, "copy")
		copyHere(t, data)
		if jsonOut {
			printJSON(copied{Seq: seq, Type: t, Size: len(data)})
		}
	},
}

// copied is what mg copy prints with --json.
type copied struct {
	Seq  uint64     `json:"seq"` // its number in the history
	Type types.MIME `json:"type"`
	Size int        `json:"size"`
}

// pasteCmd prints the person's clipboard.
var pasteCmd = &cobra.Command{
	Use:   "paste",
	Short: "Print your clipboard, the newest copy from any of your devices",
	Long: `Print your clipboard: the newest copy from any of your devices.

Text is printed as it is, and an image as its PNG bytes, which belong in a
file: mg paste > shot.png. With --json, as {"type": ..., "data": ...}, an image
in base64. One of your devices must be online, unless the copy is still on its
way to them.`,
	Args: cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		t, data, err := client.Clipboard(personKey())
		exitOn(err, "read your clipboard")
		printCopy(t, data)
	},
}

// printCopy prints a copy: text as it is, an image as PNG bytes, which it
// will not write to a terminal; with --json, as the API encodes it.
func printCopy(t types.MIME, data []byte) {
	if jsonOut {
		d := types.ClipboardData{Type: t, Data: string(data)}
		if t == types.MIMEImagePNG {
			d.Data = base64.StdEncoding.EncodeToString(data)
		}
		printJSON(d)
		return
	}
	if t == types.MIMEImagePNG && isTerminal(os.Stdout) {
		fail(exitUsage, "it is an image; send it to a file: mg paste > image.png")
	}
	os.Stdout.Write(data)
	if t != types.MIMEImagePNG && isTerminal(os.Stdout) && !bytes.HasSuffix(data, []byte("\n")) {
		fmt.Println() // leave the prompt on a line of its own; a pipe gets the bytes exactly
	}
}

// isTerminal reports whether f is a terminal rather than a pipe or a file.
// /dev/null is a character device too, but not a terminal.
func isTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }
