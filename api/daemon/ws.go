// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"github.com/gorilla/websocket"
)

// keepalive is how the daemon keeps its connection to the server alive.
type keepalive struct {
	// ping is how often the daemon pings the server. A connection that
	// stops answering — a network change, a VPN toggled, a laptop waking
	// up — would otherwise look open forever, because nothing is ever read
	// from it to find out.
	ping time.Duration
	// wait is how long the connection may stay silent before the daemon
	// gives up on it and reconnects. Any message, ping, or pong from the
	// server counts.
	wait time.Duration
	// write bounds a single write.
	write time.Duration
	// retryMin and retryMax bound the wait between reconnection attempts,
	// which doubles after each failure.
	retryMin, retryMax time.Duration
}

var defaultKeepalive = keepalive{
	ping:     30 * time.Second,
	wait:     90 * time.Second,
	write:    10 * time.Second,
	retryMin: time.Second,
	retryMax: time.Minute,
}

// subscribeURL is the websocket endpoint of the midgard server.
func (m *Daemon) subscribeURL() string {
	if m.url != "" {
		return m.url
	}
	return types.EndpointSubscribe()
}

// dial connects to the midgard server and registers the daemon. It returns
// the connection once the server has confirmed the registration.
func (m *Daemon) dial(ctx context.Context) (*websocket.Conn, error) {
	creds := config.Get().Server.Auth.User + ":" + config.Get().Server.Auth.Pass
	token := base64.StdEncoding.EncodeToString(utils.StringToBytes(creds))
	h := http.Header{"Authorization": {"Basic " + token}}

	api := m.subscribeURL()
	slog.Info("connecting to the midgard server", "api", api)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, api, h)
	if err != nil {
		return nil, fmt.Errorf("failed to connect midgard server: %w", err)
	}

	// Bound the handshake too: a server that accepts and then says nothing
	// must not hold the daemon here.
	conn.SetWriteDeadline(time.Now().Add(m.keepalive.write))
	conn.SetReadDeadline(time.Now().Add(m.keepalive.wait))
	err = conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
		Action: types.ActionHandshakeRegister,
		UserID: m.ID,
	}).Encode())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send handshake message: %w", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read message for handshake: %w", err)
	}
	wsm := &types.WebsocketMessage{}
	if err := wsm.Decode(msg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to handshake with midgard server: %w", err)
	}
	if wsm.Action != types.ActionHandshakeReady {
		conn.Close()
		return nil, fmt.Errorf("failed to handshake with midgard server: got %q", wsm.Action)
	}
	if wsm.UserID != m.ID {
		m.ID = wsm.UserID // update local id if user id is updated
		slog.Info("the hostname conflicts, the daemon id is updated", "id", m.ID)
	}
	return conn, nil
}

// stayConnected keeps the daemon connected to the server until ctx is done:
// it connects, serves the connection until it fails, and connects again,
// waiting longer after each failed attempt.
func (m *Daemon) stayConnected(ctx context.Context) {
	wait := m.keepalive.retryMin
	for ctx.Err() == nil {
		conn, err := m.dial(ctx)
		if err != nil {
			slog.Error("cannot connect to the midgard server", "err", err, "retry_in", wait)
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			wait = min(wait*2, m.keepalive.retryMax)
			continue
		}
		slog.Info("daemon is ready", "id", m.ID)

		start := time.Now()
		err = m.serve(ctx, conn)
		if ctx.Err() != nil {
			return
		}
		slog.Error("lost the connection to the midgard server, reconnecting", "err", err)
		if time.Since(start) >= m.keepalive.retryMax {
			wait = m.keepalive.retryMin // it was up for a while; try again straight away
			continue
		}
		// A connection that dies as soon as it is made counts as a failed
		// attempt, or a server that accepts and drops would be redialed
		// in a tight loop.
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, m.keepalive.retryMax)
	}
}

// serve runs one connection until it fails or ctx is done. This goroutine is
// the connection's only writer and a second one its only reader, which is
// what gorilla/websocket allows; nothing else touches the connection.
func (m *Daemon) serve(ctx context.Context, conn *websocket.Conn) error {
	defer conn.Close()

	// Anything the server sends proves the connection is alive.
	alive := func() { conn.SetReadDeadline(time.Now().Add(m.keepalive.wait)) }
	alive()
	conn.SetPongHandler(func(string) error { alive(); return nil })
	conn.SetPingHandler(func(data string) error {
		alive()
		err := conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(m.keepalive.write))
		if errors.Is(err, websocket.ErrCloseSent) {
			return nil
		}
		return err
	})

	readErr := make(chan error, 1)
	go func() { readErr <- m.readFrom(conn, alive) }()

	ping := time.NewTicker(m.keepalive.ping)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			conn.SetWriteDeadline(time.Now().Add(m.keepalive.write))
			conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
				Action: types.ActionTerminate,
				UserID: m.ID,
			}).Encode())
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return ctx.Err()
		case err := <-readErr:
			return err
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(m.keepalive.write)); err != nil {
				return fmt.Errorf("cannot ping the server: %w", err)
			}
		case msg := <-m.writeCh:
			conn.SetWriteDeadline(time.Now().Add(m.keepalive.write))
			if err := conn.WriteMessage(websocket.BinaryMessage, msg.Encode()); err != nil {
				// The message is lost with the connection; the next
				// change to the local clipboard is sent on the next one.
				return fmt.Errorf("cannot write the message to the server: %w", err)
			}
		}
	}
}

// readFrom reads messages from the server until the connection fails.
func (m *Daemon) readFrom(conn *websocket.Conn, alive func()) error {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		alive()

		wsm := &types.WebsocketMessage{}
		if err := wsm.Decode(msg); err != nil {
			slog.Error("cannot decode the message", "err", err)
			continue
		}

		// Hand the message to everyone waiting on a reply. A reader that
		// is not listening — its request timed out — must not stop the
		// connection, so a full reader is skipped.
		m.readChs.Range(func(_, v any) bool {
			select {
			case v.(chan *types.WebsocketMessage) <- wsm:
			default:
			}
			return true
		})

		switch wsm.Action {
		case types.ActionClipboardChanged:
			var d types.ClipboardData
			if err := json.Unmarshal(wsm.Data, &d); err != nil {
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
