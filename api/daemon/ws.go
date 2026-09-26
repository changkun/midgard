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
	"net/http"
	"strings"
	"time"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"github.com/gorilla/websocket"
)

func (m *Daemon) wsConnect() error {
	m.Lock()
	defer m.Unlock()

	// connect to midgard server via websocket
	creds := config.Get().Server.Auth.User + ":" + config.Get().Server.Auth.Pass
	token := base64.StdEncoding.EncodeToString(utils.StringToBytes(creds))
	h := http.Header{"Authorization": {"Basic " + token}}

	api := types.EndpointSubscribe()
	if strings.Contains(config.Get().Domain, "localhost") || strings.Contains(config.Get().Domain, "0.0.0.0") {
		api = "ws://" + api
	} else {
		api = "wss://" + api
	}
	slog.Info("connecting to the midgard server", "api", api)
	conn, _, err := websocket.DefaultDialer.Dial(api, h)
	if err != nil {
		return fmt.Errorf("failed to connect midgard server: %w", err)
	}

	m.ws = conn

	// handshake with midgard server
	err = m.ws.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
		Action: types.ActionHandshakeRegister,
		UserID: m.ID,
		Data:   nil,
	}).Encode())
	if err != nil {
		return fmt.Errorf("failed to send handshake message: %w", err)
	}
	_, msg, err := m.ws.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read message for handshake: %w", err)
	}

	wsm := &types.WebsocketMessage{}
	err = wsm.Decode(msg)
	if err != nil {
		return fmt.Errorf("failed to handshake with midgard server: %w", err)
	}

	switch wsm.Action {
	case types.ActionHandshakeReady:
		if wsm.UserID != m.ID {
			m.ID = wsm.UserID // update local id if user id is updated
			slog.Info("the hostname conflicts, the daemon id is updated", "id", m.ID)
		}
	default:
		conn.Close() // close the connection if handshake is not ready
		return fmt.Errorf("failed to handshake with midgard server: %w", err)
	}
	return nil
}

// wsReconnect tries to reconnect to the midgard server and returns
// until it connects to the server.
func (m *Daemon) wsReconnect(ctx context.Context) {
	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			err := m.wsConnect()
			if err == nil {
				slog.Info("connected to the midgard server")
				m.forceUpdate <- struct{}{}
				return
			}
			slog.Error("cannot connect to the midgard server",
				"err", err, "retry_in", 10*time.Second)
		}
	}
}

func (m *Daemon) handleIO(ctx context.Context) {
	if m.ws == nil {
		m.wsReconnect(ctx)
	}

	slog.Info("daemon is ready", "id", m.ID)
	go m.readFromServer(ctx)
	m.writeToServer(ctx)
	m.wsClose()
}

func (m *Daemon) readFromServer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, msg, err := m.ws.ReadMessage()
			if err != nil {
				slog.Error("cannot read a message from the clipboard channel", "err", err)

				m.Lock()
				m.ws = nil
				m.Unlock()

				m.wsReconnect(ctx) // block until connection is ready again
				continue
			}

			wsm := &types.WebsocketMessage{}
			err = wsm.Decode(msg)
			if err != nil {
				slog.Error("cannot decode the message", "err", err)
				continue
			}

			// duplicate messages to all readers, readers should not edit the message
			m.readChs.Range(func(k, v any) bool {
				readerCh := v.(chan *types.WebsocketMessage)
				readerCh <- wsm
				return true
			})
			switch wsm.Action {
			case types.ActionClipboardChanged:
				var d types.ClipboardData
				err = json.Unmarshal(wsm.Data, &d)
				if err != nil {
					slog.Error("cannot parse the clipboard data", "err", err)
					continue
				}
				var raw []byte
				if d.Type == types.MIMEImagePNG {
					// We assume the server send us a base64 encoded image data,
					// Let's decode it into bytes.
					raw, err = base64.StdEncoding.DecodeString(d.Data)
					if err != nil {
						raw = []byte{}
					}
				} else {
					raw = utils.StringToBytes(d.Data)
				}

				slog.Info("the universal clipboard changed, syncing with the local one",
					"from", wsm.UserID, "mime", d.Type)
				clipboard.Local.Write(d.Type, raw) // change local clipboard
			}
		}
	}
}

func (m *Daemon) writeToServer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			if m.ws == nil {
				slog.Warn("the connection was not ready")
			}
			return
		case msg := <-m.writeCh:
			if m.ws == nil {
				slog.Warn("the connection is not ready yet")
				continue
			}
			err := m.ws.WriteMessage(websocket.BinaryMessage, msg.Encode())
			if err != nil {
				slog.Error("cannot write the message to the server", "err", err)
				return
			}
		}
	}
}

func (m *Daemon) wsClose() {
	_ = m.ws.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
		Action: types.ActionTerminate,
		UserID: m.ID,
	}).Encode())

	h := m.ws.CloseHandler()
	h(websocket.CloseNormalClosure, "")

	m.ws.Close()
}
