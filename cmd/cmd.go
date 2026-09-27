// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Execute executes the midgard commands.
func Execute() {
	setupLogging()

	var r = &cobra.Command{
		Use:   "mg",
		Short: "midgard keeps your clipboard the same on all your devices.",
		Long: `midgard keeps your clipboard the same on all your devices. Your copies are
on your devices; your server passes them between them and keeps none.

For scripts and agents, --json prints results as JSON, and every command
exits with 0 on success, 1 when the server or the network failed, 2 when it
was used wrongly, 3 when this device is not signed in, 4 when none of your
devices is online to answer, 5 when there is no such thing, and 6 when this
device does not have your key (mg pair).

See https://changkun.de/s/midgard for more.
`,
	}
	r.PersistentFlags().BoolVar(&jsonOut, "json", false, "print results as JSON, for scripts and agents")

	r.AddCommand(
		versionCmd,
		loginCmd,
		logoutCmd,
		copyCmd,
		pasteCmd,
		historyCmd,
		serverCmd,
		daemonCmd,
		devicesCmd,
		queueCmd,
		shareCmd,
		sharesCmd,
		pairCmd,
		statusCmd,
	)
	r.Execute()
}

// setupLogging installs the process-wide slog handler.
func setupLogging() {
	slog.SetDefault(slog.New(newLogHandler(os.Stderr)))
}

// newLogHandler builds the midgard log handler. The source attribute is
// trimmed to file:line because the absolute build path adds no information
// to a log line that already names the file.
func newLogHandler(w io.Writer) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key != slog.SourceKey {
				return a
			}
			if src, ok := a.Value.Any().(*slog.Source); ok {
				src.File = filepath.Base(src.File)
			}
			return a
		},
	})
}

// errorf reports a message to the person running the command. Command output
// is not operator logging, so it carries no timestamp, level, or source, and
// it goes to stderr to keep stdout free for the data the command produces.
func errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
