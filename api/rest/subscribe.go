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
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var uid atomic.Uint64 // atomic, incremental

// keepalive is how the server notices daemons that went away without closing
// their connection.
type keepalive struct {
	// ping is how often the server pings each daemon; a daemon answers
	// from its read loop, including daemons built before they sent pings
	// of their own.
	ping time.Duration
	// wait is how long a daemon may stay silent before the server drops
	// it. Without it a dead daemon stayed listed forever, and every
	// broadcast kept writing to it.
	wait time.Duration
	// write bounds a single write, so one stuck daemon cannot hold up a
	// broadcast to all the others.
	write time.Duration
}

var defaultKeepalive = keepalive{ping: 30 * time.Second, wait: 90 * time.Second, write: 10 * time.Second}

// user represents a daemon subscriber
type user struct {
	sync.Mutex
	index uint64
	id    string
	owner string // whose daemon it is; it hears only its owner's copies
	conn  *websocket.Conn
	write time.Duration // bounds each send
}

func (d *user) send(msg *types.WebsocketMessage) error {
	d.Lock()
	defer d.Unlock()

	if d.conn == nil {
		return errors.New("sender connection was closed")
	}
	d.conn.SetWriteDeadline(time.Now().Add(d.write))
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
		// The daemon joins its owner's room, named by its sign-in and not
		// by anything it says.
		owner = c.GetString(ctxOwner)
	)
	switch wsm.Action {
	case types.ActionHandshakeRegister:
		// check if user id already exist
		idExist := false
		m.mu.Lock()
		for e := m.users.Front(); e != nil; e = e.Next() {
			u, ok := e.Value.(*user)
			if !ok || u.owner != owner || u.id != wsm.UserID {
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
		u = &user{index: idx, id: wsm.UserID, owner: owner, conn: conn, write: m.keepalive.write}
		e = m.users.PushBack(u)
		slog.Info("a daemon subscribed", "subscribers", m.users.Len())
		m.mu.Unlock()

		// send confirmation. From here on every write goes through
		// u.send: a broadcast can be writing to this connection at the same
		// time, and gorilla/websocket allows one writer at a time.
		err := u.send(&types.WebsocketMessage{
			Action: types.ActionHandshakeReady, UserID: u.id,
		})
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

	// Anything the daemon sends proves it is still there; pings go out
	// on their own, as control frames may be written alongside u.send.
	ka := m.keepalive
	alive := func() { conn.SetReadDeadline(time.Now().Add(ka.wait)) }
	alive()
	conn.SetPongHandler(func(string) error { alive(); return nil })
	conn.SetPingHandler(func(data string) error {
		alive()
		err := conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(ka.write))
		if errors.Is(err, websocket.ErrCloseSent) {
			return nil
		}
		return err
	})
	stopPing := make(chan struct{})
	defer close(stopPing)
	go func() {
		t := time.NewTicker(ka.ping)
		defer t.Stop()
		for {
			select {
			case <-stopPing:
				return
			case <-t.C:
				if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(ka.write)) != nil {
					return
				}
			}
		}
	}()

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
		alive()

		wsm := &types.WebsocketMessage{}
		err = wsm.Decode(msg)
		if err != nil {
			// send a bad format message
			// we con't care about the error here (?)
			u.send(&types.WebsocketMessage{
				Action:  types.ActionTerminate,
				Message: "bad message format",
			})
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
		default:
			slog.Warn("unsupported message",
				"action", wsm.Action, "msg", utils.BytesToString(msg))
		}
	}
}

func terminate(u *user, err error) error {
	u.send(&types.WebsocketMessage{
		Action:  types.ActionTerminate,
		Message: "bad action data",
	})
	return fmt.Errorf("bad action: %w", err)
}

// Devices lists the daemons connected to the server. It is a plain request,
// which mg daemon ls and mg status make; it used to be a round trip over a
// daemon's websocket, relayed to the command by the daemon's local RPC.
func (m *Midgard) Devices(c *gin.Context) {
	owner := c.GetString(ctxOwner)
	m.mu.Lock()
	out := types.DevicesOutput{Devices: []types.Device{}}
	for e := m.users.Front(); e != nil; e = e.Next() {
		if u := e.Value.(*user); u.owner == owner {
			out.Devices = append(out.Devices, types.Device{Index: u.index, Name: u.id})
		}
	}
	m.mu.Unlock()
	c.JSON(http.StatusOK, out)
}

func (m *Midgard) handleActionClipboardPut(conn *websocket.Conn, u *user, data []byte) error {
	b := &types.PutToUniversalClipboardInput{}
	err := json.Unmarshal(data, b)
	if err != nil {
		_ = u.send(&types.WebsocketMessage{
			Action:  types.ActionTerminate,
			Message: "bad action data",
		})
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

	updated := clipboard.UniversalFor(u.owner).Write(b.Type, raw)
	slog.Info("the universal clipboard is updated", "from", u.id)
	if updated {
		// Include MIME type information so that the clipboard is
		// consistent after sync propagation.
		raw, _ = json.Marshal(b.ClipboardData)
		m.boardcastMessage(u.owner, &types.WebsocketMessage{
			Action:  types.ActionClipboardChanged,
			UserID:  u.id,
			Message: "universal clipboard has changes",
			Data:    raw, // clipboard data
		})
	}
	return nil
}

// boardcastMessage sends msg to owner's daemons, but the one it came from.
// A copy never leaves its owner's room.
func (m *Midgard) boardcastMessage(owner string, msg *types.WebsocketMessage) {
	slog.Info("broadcasting a message", "from", msg.UserID)
	m.mu.Lock()
	for e := m.users.Front(); e != nil; e = e.Next() {
		d, ok := e.Value.(*user)
		if !ok || d.owner != owner || d.id == msg.UserID {
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
