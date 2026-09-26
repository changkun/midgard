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
		Short: "midgard is a universal clipboard service.",
		Long: `midgard is a universal clipboard service.
See https://changkun.de/s/midgard for more details.
`,
	}

	r.AddCommand(
		versionCmd,
		loginCmd,
		logoutCmd,
		serverCmd,
		daemonCmd,
		allocCmd,
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
