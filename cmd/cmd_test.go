// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestLogHandlerTrimsSource pins the source attribute to a bare file:line.
// A full build path leaks the machine that compiled the binary into every
// log line and pushes the message off the terminal.
func TestLogHandlerTrimsSource(t *testing.T) {
	var buf bytes.Buffer
	slog.New(newLogHandler(&buf)).Info("hello", "key", "value")

	got := buf.String()
	if !strings.Contains(got, "source=cmd_test.go:") {
		t.Errorf("log line has no trimmed source: %q", got)
	}
	if strings.Contains(got, "source=/") {
		t.Errorf("log line keeps an absolute source path: %q", got)
	}
	if !strings.Contains(got, `msg=hello`) || !strings.Contains(got, `key=value`) {
		t.Errorf("log line lost the message or the attribute: %q", got)
	}
}
