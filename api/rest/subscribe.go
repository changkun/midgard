// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rest

import (
	"container/list"
	"encoding/json"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.design/x/midgard/pkg/types"
	"golang.design/x/midgard/pkg/utils"
)

// daemon represents a daemon subscriber
type daemon struct {
	id   string
	conn *websocket.Conn
}

func (d *daemon) notify(a types.SubscribeAction) {
	if d.conn == nil {
		return
	}

	sm := types.SubscribeMessage{
		Action:   types.ActionClipboardChanged,
		DaemonID: d.id,
	}
	d.conn.WriteMessage(websocket.BinaryMessage, sm.Encode())
}

// SubscribeClipboard subscribes the Midgard's universal clipboard.
func (m *Midgard) SubscribeClipboard(c *gin.Context) {
	// upgrade connection
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// read messages from socket
	var e *list.Element
	_, msg, err := conn.ReadMessage()

	var sm types.SubscribeMessage
	err = json.Unmarshal(msg, &sm)
	if err != nil {
		conn.WriteMessage(websocket.BinaryMessage, (&types.SubscribeMessage{
			Action:  types.ActionTerminate,
			Message: "invalid message format",
		}).Encode())
		conn.Close()
		return
	}

	switch sm.Action {
	case types.ActionRegister:

		// register to the subscribers
		m.mu.Lock()
		d := &daemon{
			id:   utils.NewUUID(),
			conn: conn,
		}
		e = m.daemons.PushBack(d)
		log.Printf("current daemon subscribers: %d", m.daemons.Len())
		m.mu.Unlock()

		// send confirmation
		conn.WriteMessage(websocket.BinaryMessage, (&types.SubscribeMessage{
			Action:   types.ActionReady,
			DaemonID: d.id,
		}).Encode())

	default:
		conn.WriteMessage(websocket.BinaryMessage, (&types.SubscribeMessage{
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
			m.daemons.Remove(e)
			log.Printf("remaining daemon subscribers: %d", m.daemons.Len())
			m.mu.Unlock()
			conn.Close()
			return
		}
		log.Println(utils.BytesToString(msg))
	}
}

func (m *Midgard) notifyOtherDaemons(id string, a types.SubscribeAction) {
	log.Println("clipboard notifier: ", id)
	m.mu.Lock()
	for e := m.daemons.Front(); e != nil; e = e.Next() {
		d := e.Value.(*daemon)
		if d.id != id {
			log.Println("notified to: ", d.id)
			d.notify(a)
		}
	}
	m.mu.Unlock()
}
