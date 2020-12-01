// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rpc

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

func (m *Midgard) wsConnect() error {
	// connect to midgard server via websocket
	creds := config.Get().Server.Auth.User + ":" + config.Get().Server.Auth.Pass
	token := base64.StdEncoding.EncodeToString(utils.StringToBytes(creds))
	h := http.Header{"Authorization": {"Basic " + token}}

	api := types.ClipboardWSEndpoint
	if strings.Contains(config.Get().Domain, "localhost") {
		api = "ws://" + api
	} else {
		api = "wss://" + api
	}
	conn, _, err := websocket.DefaultDialer.Dial(api, h)
	if err != nil {
		return fmt.Errorf("failed to connect midgard server: %w", err)
	}

	m.Lock()
	m.ws = conn
	m.Unlock()

	// handshake
	m.ws.WriteMessage(websocket.BinaryMessage, (&types.SubscribeMessage{
		Action: types.ActionRegister,
	}).Encode())
	_, msg, err := m.ws.ReadMessage()
	var sm types.SubscribeMessage
	err = json.Unmarshal(msg, &sm)
	if err != nil {
		return fmt.Errorf("failed to handhsake with midgard server: %w", err)
	}
	m.Lock()
	m.id = sm.DaemonID
	m.Unlock()

	return nil
}

func (m *Midgard) wsClose() {
	m.ws.Close()
}

// wsReconnect tries to reconnect to the midgard server and returns
// until it connects to the server.
func (m *Midgard) wsReconnect() {
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

func (m *Midgard) wsListen() {
	if m.ws == nil {
		m.wsReconnect()
	}

	log.Println("daemon id:", m.id)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		m.readchan()
		wg.Done()
	}()
	go func() {
		m.writechan()
		wg.Done()
	}()
	wg.Wait()
}

func (m *Midgard) readchan() {
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
		var sm types.SubscribeMessage
		err = json.Unmarshal(msg, &sm)
		if err != nil {
			log.Printf("failed to read message: %v", err)
			continue
		}
		switch sm.Action {
		case types.ActionClipboardChanged:
			// read from universal
			res, err := utils.Request(http.MethodGet, types.ClipboardEndpoint, nil)
			if err != nil {
				log.Printf("failed to read universal clipboard: %v", err)
				continue
			}
			var out types.GetFromUniversalClipboardOutput
			err = json.Unmarshal(res, &out)
			if err != nil {
				log.Printf("failed to parse clipboard data: %v", err)
				continue
			}

			// decode and write to local
			var raw []byte
			if out.Type == types.ClipboardDataTypeImagePNG {
				raw, err = base64.StdEncoding.DecodeString(out.Data)
				if err != nil {
					raw = []byte{}
				}
			} else {
				raw = utils.StringToBytes(out.Data)
			}
			clipboard.Write(raw)
		}
	}
}
func (m *Midgard) writechan() {
	for {
		select {
		case act := <-m.writeCh:
			fmt.Println(act)
			m.ws.WriteMessage(websocket.BinaryMessage, (&types.SubscribeMessage{
				Action: types.ActionGetClipboard,
			}).Encode())
		}
	}
}

func (m *Midgard) watchLocalClipboard(ctx context.Context) {
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

			// TODO: write clipboard data using websocket action
			d := &types.PutToUniversalClipboardInput{}
			d.Type = types.ClipboardDataTypePlainText
			d.Data = utils.BytesToString(text)
			d.DaemonID = m.id
			_, err := utils.Request(http.MethodPost, types.ClipboardEndpoint, d)
			if err != nil {
				log.Printf("failed to sync clipboard, err: %v", err)
			}
		case img, ok := <-imagCh:
			if !ok {
				return
			}
			d := &types.PutToUniversalClipboardInput{}
			d.Type = types.ClipboardDataTypeImagePNG
			d.Data = base64.StdEncoding.EncodeToString(img)
			d.DaemonID = m.id

			// TODO: write clipboard data using websocket action
			_, err := utils.Request(http.MethodPost, types.ClipboardEndpoint, d)
			if err != nil {
				log.Printf("failed to sync clipboard, err: %v", err)
			}
		}
	}
}
