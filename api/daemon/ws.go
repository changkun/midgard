// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
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

	api := types.EndpointSubscribe
	if strings.Contains(config.Get().Domain, "localhost") {
		api = "ws://" + api
	} else {
		api = "wss://" + api
	}
	log.Println("connecting to:", api)
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
	wsm := &types.WebsocketMessage{}
	err = wsm.Decode(msg)
	if err != nil {
		return fmt.Errorf("failed to handhsake with midgard server: %w", err)
	}

	switch wsm.Action {
	case types.ActionHandshakeReady:
		if wsm.UserID != m.ID {
			m.ID = wsm.UserID // update local id if user id is updated
			log.Println("conflict hostname, updated daemon id: ", m.ID)
		}
	default:
		conn.Close() // close the connection if handshake is not ready
		return fmt.Errorf("failed to handhsake with midgard server: %w", err)
	}
	return nil
}

func (m *Daemon) wsClose() {
	m.ws.Close()
}

// wsReconnect tries to reconnect to the midgard server and returns
// until it connects to the server.
func (m *Daemon) wsReconnect() {
	for {
		time.Sleep(time.Second * 10)
		err := m.wsConnect()
		if err == nil {
			log.Println("connected to midgard server.")
			return
		}
		log.Printf("%v\n", err)
		log.Println("retry in 10 seconds..")
	}
}

func (m *Daemon) handleIO(ctx context.Context) {
	if m.ws == nil {
		m.wsReconnect()
	}

	log.Println("daemon id:", m.ID)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() { // read from server
		defer wg.Done()
		m.readFromServer()
	}()
	go func() { // write to server
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				if m.ws == nil {
					log.Println("connection is not ready yet")
					continue
				}
				_ = m.ws.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
					Action: types.ActionTerminate,
					UserID: m.ID,
				}).Encode())
				return
			case msg := <-m.writeCh:
				if m.ws == nil {
					log.Println("connection is not ready yet")
					continue
				}
				err := m.ws.WriteMessage(websocket.BinaryMessage, msg.Encode())
				if err != nil {
					log.Printf("failed to write message to server: %v", err)
					return
				}
			}
		}
	}()
	wg.Wait()
}

func (m *Daemon) readFromServer() {
	for {
		_, msg, err := m.ws.ReadMessage()
		if err != nil {
			log.Printf("failed to read message from the clipboard channel: %v", err)

			m.Lock()
			m.ws = nil
			m.Unlock()

			m.wsReconnect() // block until connection is ready again
			continue
		}

		wsm := &types.WebsocketMessage{}
		err = wsm.Decode(msg)
		if err != nil {
			log.Printf("failed to read message: %v", err)
			continue
		}

		// duplicate messages to all readers, readers should not edit the message
		m.readChs.Range(func(k, v interface{}) bool {
			// readerID := k.(string)
			readerCh := v.(chan *types.WebsocketMessage)
			readerCh <- wsm
			return true
		})
		switch wsm.Action {
		case types.ActionClipboardChanged:
			var d types.ClipboardData
			err = json.Unmarshal(wsm.Data, &d)
			if err != nil {
				log.Printf("failed to parse clipboard data: %v", err)
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

			log.Printf("universal clipboard has changed from %s, sync with local...", wsm.UserID)
			clipboard.Local.Write(d.Type, raw) // change local clipboard
		}
	}
}
