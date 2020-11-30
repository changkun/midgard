// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"golang.design/x/midgard/pkg/clipboard"
	"golang.design/x/midgard/pkg/config"
	"golang.design/x/midgard/pkg/types"
	"golang.design/x/midgard/pkg/utils"
)

func (m *Midgard) wsConnect() {
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
		log.Print("failed to connect clipboard channel", err)
		return
	}

	m.ws = conn
}

func (m *Midgard) wsClose() {
	m.ws.Close()
}

func (m *Midgard) wsHandshake() {
	m.ws.WriteMessage(websocket.BinaryMessage, (&types.SubscribeMessage{
		Action: types.ActionRegister,
	}).Encode())
	_, msg, err := m.ws.ReadMessage()
	var sm types.SubscribeMessage
	err = json.Unmarshal(msg, &sm)
	if err != nil {
		log.Printf("failed to on handhsake phase: %v", err)
		return
	}
	m.Lock()
	m.id = sm.DaemonID
	m.Unlock()
}

func (m *Midgard) wsListen() {
	log.Println("daemon id:", m.id)
	for {
		_, msg, err := m.ws.ReadMessage()
		if err != nil {
			log.Printf("failed to read message from the clipboard channel: %v", err)
			return
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

			_, err := utils.Request(http.MethodPost, types.ClipboardEndpoint, d)
			if err != nil {
				log.Printf("failed to sync clipboard, err: %v", err)
			}
		}
	}
}
