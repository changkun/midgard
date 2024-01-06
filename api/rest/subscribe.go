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
	"log"
	"sync"
	"sync/atomic"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var uid uint64 // atomic, incremental

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
		log.Printf("failed to upgrade the connection: %v", err)
		return
	}

	// read messages from socket
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Printf("failed to read message from connection: %v", err)
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
		log.Printf("failed to parse handshake information: %v", err)
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
		idx := atomic.AddUint64(&uid, 1)
		u = &user{index: idx, id: wsm.UserID, conn: conn}
		e = m.users.PushBack(u)
		log.Printf("current daemon subscribers: %d", m.users.Len())
		m.mu.Unlock()

		// send confirmation
		err := conn.WriteMessage(
			websocket.BinaryMessage, (&types.WebsocketMessage{
				Action: types.ActionHandshakeReady, UserID: u.id,
			}).Encode())
		if err != nil {
			log.Printf("failed in register handshake: %v", err)
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
			log.Printf("remaining daemon subscribers: %d", n)
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
			log.Println("put clipboard request is received.")
			err := m.handleActionClipboardPut(conn, u, wsm.Data)
			if err != nil {
				log.Println("failed to put clipboard:", err)
			}
		case types.ActionListDaemonsRequest:
			log.Println("list active daemons request is received.")
			err := m.handleListDaemons(conn, u, wsm.Data)
			if err != nil {
				log.Println("failed to list daemons:", err)
			}
		default:
			log.Printf("unsupported message: action(%v), msg(%v)", wsm.Action, utils.BytesToString(msg))
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

	resp := "id\tname\n"

	for e := m.users.Front(); e != nil; e = e.Next() {
		u := e.Value.(*user)
		resp += fmt.Sprintf("%d\t%s\n", u.index, u.id)
	}

	return conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
		Action: types.ActionListDaemonsResponse,
		Data:   utils.StringToBytes(resp),
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
	log.Println("universal clipboard has updated, synced from:", u.id)
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
	log.Println("broadcast message from:", msg.UserID)
	m.mu.Lock()
	for e := m.users.Front(); e != nil; e = e.Next() {
		d, ok := e.Value.(*user)
		if !ok || d.id == msg.UserID {
			continue
		}
		log.Println("send message to:", d.id)
		err := d.send(msg)
		if err != nil {
			log.Printf("failed to send to %s, err: %v\n", d.id, err)
		}
	}
	m.mu.Unlock()
	log.Println("broadcast message is finished.")
}
