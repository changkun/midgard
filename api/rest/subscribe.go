// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rest

import (
	"container/list"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.design/x/midgard/pkg/clipboard"
	"golang.design/x/midgard/pkg/types"
	"golang.design/x/midgard/pkg/utils"
)

// user represents a daemon subscriber
type user struct {
	sync.Mutex
	id   string
	conn *websocket.Conn
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

	id := wsm.UserID
	var e *list.Element
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
			id += "-" + utils.NewUUID()
		}

		// register to the subscribers
		u := &user{id: id, conn: conn}
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
		case types.ActionClipboardPut:
			b := &types.PutToUniversalClipboardInput{}
			err := json.Unmarshal(wsm.Data, b)
			if err != nil {
				conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
					Action:  types.ActionTerminate,
					Message: "bad action data",
				}).Encode())
				continue
			}
			var raw []byte
			if b.Type == types.ClipboardDataTypeImagePNG {
				// We assume the client send us a base64 encoded image data,
				// Let's decode it into bytes.
				raw, err = base64.StdEncoding.DecodeString(b.Data)
				if err != nil {
					raw = []byte{}
				}
			} else {
				raw = utils.StringToBytes(b.Data)
			}
			log.Println("universal clipboard has updated, synced from:", id)
			updated := clipboard.Universal.Put(b.Type, raw)
			if updated {
				m.boardcastMessage(id, &types.WebsocketMessage{
					Action:  types.ActionClipboardChanged,
					UserID:  id,
					Message: "universal clipboard has changes",
					Data:    raw, // clipboard data
				})
			}
		default:
			log.Println("unsupported message:", utils.BytesToString(msg))
		}
	}
}

func (m *Midgard) boardcastMessage(senderID string, msg *types.WebsocketMessage) {
	log.Println("broadcast message from:", senderID)
	m.mu.Lock()
	for e := m.users.Front(); e != nil; e = e.Next() {
		d, ok := e.Value.(*user)
		if !ok || d.id == senderID {
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
