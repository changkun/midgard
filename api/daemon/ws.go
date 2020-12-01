// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

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

	"github.com/gorilla/websocket"
	"golang.design/x/midgard/pkg/clipboard"
	"golang.design/x/midgard/pkg/config"
	"golang.design/x/midgard/pkg/types"
	"golang.design/x/midgard/pkg/utils"
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

		switch wsm.Action {
		case types.ActionClipboardChanged:
			log.Printf("universal clipboard has changed from %s, sync with local...", wsm.UserID)
			clipboard.Write(wsm.Data) // change local clipboard
		}
	}
}

func (m *Daemon) watchLocalClipboard(ctx context.Context) {
	textCh := make(chan []byte, 1)
	clipboard.Watch(ctx, types.ClipboardDataTypePlainText, textCh)
	imagCh := make(chan []byte, 1)
	clipboard.Watch(ctx, types.ClipboardDataTypeImagePNG, imagCh)
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
			d.Type = types.ClipboardDataTypePlainText
			d.Data = utils.BytesToString(text)
			d.DaemonID = m.ID
			b, _ := json.Marshal(d)
			log.Println("local clipboard has changed, sync to server...")
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
			d.Type = types.ClipboardDataTypeImagePNG
			d.Data = base64.StdEncoding.EncodeToString(img)
			d.DaemonID = m.ID
			b, _ := json.Marshal(d)
			log.Println("local clipboard has changed, sync to server...")
			m.writeCh <- &types.WebsocketMessage{
				Action:  types.ActionClipboardPut,
				UserID:  m.ID,
				Message: "local clipboard has changed",
				Data:    b,
			}
		}
	}
}
