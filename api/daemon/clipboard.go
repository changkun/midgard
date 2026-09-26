// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/hotkey"
	"changkun.de/x/midgard/internal/types"
)

// watchLocalClipboard records every copy made on this device in its history,
// which sends it to the person's other devices, until ctx is done. Copies
// marked sensitive never get here (internal/clipboard drops them), and what
// the daemon put on the clipboard itself is not sent back.
func (m *Daemon) watchLocalClipboard(ctx context.Context) {
	last := time.Now()
	hotkey.Handle(ctx, func() {
		if time.Since(last) < time.Second*5 {
			slog.Warn("the hotkey was pressed too fast, ignoring it")
			return
		}
		last = time.Now()
		slog.Info("the hotkey is triggered")
		msg := m.share()
		slog.Info(msg)
		clipboard.Local.Write(types.MIMEPlainText, []byte(msg))
	})

	textCh := clipboard.Local.Watch(ctx, types.MIMEPlainText)
	imagCh := clipboard.Local.Watch(ctx, types.MIMEImagePNG)
	for {
		var mime types.MIME
		var data []byte
		var ok bool
		select {
		case <-ctx.Done():
			return
		case data, ok = <-textCh:
			mime = types.MIMEPlainText
		case data, ok = <-imagCh:
			mime = types.MIMEImagePNG
		}
		if !ok {
			return
		}
		if len(data) == 0 || string(data) == "\n" || m.ours(data) {
			continue
		}
		slog.Info("the local clipboard changed", "mime", mime, "online", m.engine.Online())
		if err := m.engine.Copy(ctx, string(mime), data); err != nil {
			slog.Error("cannot record the copy", "err", err)
		}
	}
}

// share publishes what is on the local clipboard at a link, and says what
// to put on the clipboard: the link, or why there is none.
func (m *Daemon) share() string {
	_, data := clipboard.Local.Read()
	if len(data) == 0 {
		return "there is nothing on the clipboard to share"
	}
	sh, err := client.Share("", data, "", 0)
	if err != nil {
		return fmt.Sprintf("cannot share the clipboard: %v", err)
	}
	return sh.URL
}
