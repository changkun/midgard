// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"container/list"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var uid atomic.Uint64 // atomic, incremental

// user represents a daemon subscriber
type user struct {
	sync.Mutex
	index uint64
	id    string
	conn  *websocket.Conn
}

func (d *user) send(msg *types.WebsocketMessage) error {
	d.Lock()
	defer d.Unlock()

	if d.conn == nil {
		return errors.New("sender connection was closed")
	}
	return d.conn.WriteMessage(websocket.BinaryMessage, msg.Encode())
}

// Subscribe subscribes the Midgard's server.
func (m *Midgard) Subscribe(c *gin.Context) {
	// upgrade connection
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("cannot upgrade the connection", "err", err)
		return
	}

	// read messages from socket
	_, msg, err := conn.ReadMessage()
	if err != nil {
		slog.Error("cannot read a message from the connection", "err", err)
		return
	}
	wsm := &types.WebsocketMessage{}
	err = json.Unmarshal(msg, wsm)
	if err != nil {
		// we con't care about the error here (?)
		conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
			Action:  types.ActionTerminate,
			Message: "invalid message format",
		}).Encode())
		conn.Close()
		slog.Error("cannot parse the handshake information", "err", err)
		return
	}

	var (
		u *user
		e *list.Element
	)
	switch wsm.Action {
	case types.ActionHandshakeRegister:
		// check if user id already exist
		idExist := false
		m.mu.Lock()
		for e := m.users.Front(); e != nil; e = e.Next() {
			u, ok := e.Value.(*user)
			if !ok || u.id != wsm.UserID {
				continue
			}
			idExist = true
			break
		}
		if idExist {
			id, err := utils.NewUUIDShort()
			if err != nil {
				panic(fmt.Errorf("failed to create a new uuid: %v", err))
			}
			wsm.UserID += "-" + id
		}

		// register to the subscribers
		idx := uid.Add(1)
		u = &user{index: idx, id: wsm.UserID, conn: conn}
		e = m.users.PushBack(u)
		slog.Info("a daemon subscribed", "subscribers", m.users.Len())
		m.mu.Unlock()

		// send confirmation
		err := conn.WriteMessage(
			websocket.BinaryMessage, (&types.WebsocketMessage{
				Action: types.ActionHandshakeReady, UserID: u.id,
			}).Encode())
		if err != nil {
			slog.Error("cannot complete the register handshake", "err", err)
			return
		}
	default:
		// we con't care about the error here (?)
		conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
			Action:  types.ActionTerminate,
			Message: "unsupported action",
		}).Encode())
		conn.Close()
		return
	}

	// start looping
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			m.mu.Lock()
			m.users.Remove(e)
			n := m.users.Len()
			m.mu.Unlock()
			slog.Info("a daemon unsubscribed", "subscribers", n)
			conn.Close()
			return
		}

		wsm := &types.WebsocketMessage{}
		err = wsm.Decode(msg)
		if err != nil {
			// send a bad format message
			// we con't care about the error here (?)
			conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
				Action:  types.ActionTerminate,
				Message: "bad message format",
			}).Encode())
			conn.Close()
			return
		}

		switch wsm.Action {
		case types.ActionTerminate:
			continue
		case types.ActionClipboardPut:
			slog.Info("received a put clipboard request", "user", u.id)
			err := m.handleActionClipboardPut(conn, u, wsm.Data)
			if err != nil {
				slog.Error("cannot put the clipboard", "user", u.id, "err", err)
			}
		case types.ActionListDaemonsRequest:
			slog.Info("received a list active daemons request", "user", u.id)
			err := m.handleListDaemons(conn, u, wsm.Data)
			if err != nil {
				slog.Error("cannot list the daemons", "user", u.id, "err", err)
			}
		default:
			slog.Warn("unsupported message",
				"action", wsm.Action, "msg", utils.BytesToString(msg))
		}
	}
}

func terminate(conn *websocket.Conn, err error) error {
	conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
		Action:  types.ActionTerminate,
		Message: "bad action data",
	}).Encode())
	return fmt.Errorf("bad action: %w", err)
}

func (m *Midgard) handleListDaemons(conn *websocket.Conn, u *user, data []byte) (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	defer func() {
		if err != nil {
			err = terminate(conn, err)
		}
	}()

	var resp strings.Builder
	resp.WriteString("id\tname\n")

	for e := m.users.Front(); e != nil; e = e.Next() {
		u := e.Value.(*user)
		fmt.Fprintf(&resp, "%d\t%s\n", u.index, u.id)
	}

	return conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
		Action: types.ActionListDaemonsResponse,
		Data:   utils.StringToBytes(resp.String()),
	}).Encode())
}

func (m *Midgard) handleActionClipboardPut(conn *websocket.Conn, u *user, data []byte) error {
	b := &types.PutToUniversalClipboardInput{}
	err := json.Unmarshal(data, b)
	if err != nil {
		_ = conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
			Action:  types.ActionTerminate,
			Message: "bad action data",
		}).Encode())
		return types.ErrBadAction
	}
	var raw []byte
	if b.Type == types.MIMEImagePNG {
		// We assume the client send us a base64 encoded image data,
		// Let's decode it into bytes.
		raw, err = base64.StdEncoding.DecodeString(b.Data)
		if err != nil {
			raw = []byte{}
		}
	} else {
		raw = utils.StringToBytes(b.Data)
	}

	updated := clipboard.Universal.Write(b.Type, raw)
	slog.Info("the universal clipboard is updated", "from", u.id)
	if updated {
		// Include MIME type information so that the clipboard is
		// consistent after sync propagation.
		raw, _ = json.Marshal(b.ClipboardData)
		m.boardcastMessage(&types.WebsocketMessage{
			Action:  types.ActionClipboardChanged,
			UserID:  u.id,
			Message: "universal clipboard has changes",
			Data:    raw, // clipboard data
		})
	}
	return nil
}

func (m *Midgard) boardcastMessage(msg *types.WebsocketMessage) {
	slog.Info("broadcasting a message", "from", msg.UserID)
	m.mu.Lock()
	for e := m.users.Front(); e != nil; e = e.Next() {
		d, ok := e.Value.(*user)
		if !ok || d.id == msg.UserID {
			continue
		}
		slog.Info("sending a message", "to", d.id)
		err := d.send(msg)
		if err != nil {
			slog.Error("cannot send the message", "to", d.id, "err", err)
		}
	}
	m.mu.Unlock()
	slog.Info("the broadcast is finished")
}
