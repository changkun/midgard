// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package watch

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

// Clipboard listen to the clipboard for a given data
func Clipboard() {
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
	defer conn.Close()

	conn.WriteMessage(websocket.BinaryMessage, (&types.SubscribeMessage{
		Action: types.ActionRegister,
	}).Encode())
	_, msg, err := conn.ReadMessage()
	var sm types.SubscribeMessage
	err = json.Unmarshal(msg, &sm)
	if err != nil {
		log.Print("failed to connect clipboard channel", err)
		return
	}

	// message loop
	go func(id string) {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("failed to read message from the clipboard channel: %v", err)
				// TODO: reconnecting
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
				// 1. read from universal
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
	}(sm.DaemonID)
	log.Println("daemon id:", sm.DaemonID)

	// run daemon and watch clipboard data
	textCh := make(chan []byte, 1)
	clipboard.Watch(context.Background(), types.ClipboardDataTypePlainText, textCh)
	imagCh := make(chan []byte, 1)
	clipboard.Watch(context.Background(), types.ClipboardDataTypeImagePNG, imagCh)
	for {
		select {
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
			d.DaemonID = sm.DaemonID
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
			d.DaemonID = sm.DaemonID

			_, err := utils.Request(http.MethodPost, types.ClipboardEndpoint, d)
			if err != nil {
				log.Printf("failed to sync clipboard, err: %v", err)
			}
		}
	}
}
