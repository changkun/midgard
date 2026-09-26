// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/hotkey"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
)

func (m *Daemon) watchLocalClipboard(ctx context.Context) {
	last := time.Now()
	hotkey.Handle(ctx, func() {
		if time.Since(last) < time.Second*5 {
			slog.Warn("the hotkey was pressed too fast, ignoring it")
			return
		}
		last = time.Now()

		var msg string
		defer func() {
			slog.Info(msg)
			clipboard.Local.Write(
				types.MIMEPlainText, utils.StringToBytes(msg))
		}()

		slog.Info("the hotkey is triggered")
		sh, err := client.Share("", nil, "", 0)
		if err != nil {
			msg = fmt.Sprintf("cannot share the clipboard: %v", err)
			return
		}
		msg = sh.URL
	})

	textCh := clipboard.Local.Watch(ctx, types.MIMEPlainText)
	imagCh := clipboard.Local.Watch(ctx, types.MIMEImagePNG)
	for {
		select {
		case <-ctx.Done():
			return
		case text, ok := <-textCh:
			if !ok {
				return
			}

			// don't send an '\n' character
			if utils.BytesToString(text) == "\n" {
				continue
			}

			d := &types.PutToUniversalClipboardInput{}
			d.Type = types.MIMEPlainText
			d.Data = utils.BytesToString(text)
			d.DaemonID = m.ID
			b, _ := json.Marshal(d)
			slog.Info("the local clipboard changed, syncing to the server", "mime", types.MIMEPlainText)
			m.writeCh <- &types.WebsocketMessage{
				Action:  types.ActionClipboardPut,
				UserID:  m.ID,
				Message: "local clipboard has changed",
				Data:    b,
			}
		case img, ok := <-imagCh:
			if !ok {
				return
			}
			d := &types.PutToUniversalClipboardInput{}
			d.Type = types.MIMEImagePNG
			d.Data = base64.StdEncoding.EncodeToString(img)
			d.DaemonID = m.ID
			b, _ := json.Marshal(d)
			slog.Info("the local clipboard changed, syncing to the server", "mime", types.MIMEImagePNG)
			m.writeCh <- &types.WebsocketMessage{
				Action:  types.ActionClipboardPut,
				UserID:  m.ID,
				Message: "local clipboard has changed",
				Data:    b,
			}
		}
	}
}
