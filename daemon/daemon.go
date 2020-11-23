// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"encoding/base64"
	"net/http"

	"golang.design/x/midgard/clipboard"
	"golang.design/x/midgard/config"
	"golang.design/x/midgard/types"
	"golang.design/x/midgard/utils"
)

// Run runs the midgard's daemon process.
func Run() {
	// TODO: we have several remaining task for the daemon:
	//
	// 1. register a websocket connection for universal clipboard push
	// notification: if the cloud is changed, then it should notify all
	// subscribers, instead of the following deadloop:
	//
	// go func() {
	// 	url := "http://" + config.Get().Addr.HTTP + "/midgard/api/v1/clipboard"
	// 	t := time.NewTicker(time.Second * 2)
	// 	for {
	// 		select {
	// 		case <-t.C:
	// 			_, err := request(http.MethodGet, url, nil)
	// 			if err != nil {
	// 				fmt.Println(err)
	// 			}
	// 		}
	// 	}
	// }()
	//
	// 2. register to system hotkey, trigger special handlers
	watchClipboard()
}

// watchClipboard listen to the clipboard for a given data
func watchClipboard() {
	// run daemon and watch clipboard data
	textCh := make(chan []byte, 1)
	clipboard.Watch(context.Background(), types.ClipboardDataTypePlainText, textCh)
	imagCh := make(chan []byte, 1)
	clipboard.Watch(context.Background(), types.ClipboardDataTypeImagePNG, imagCh)
	url := "http://" + config.Get().Addr.HTTP + "/midgard/api/v1/clipboard"
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

			utils.Request(http.MethodPost, url, &types.ClipboardData{
				Type: types.ClipboardDataTypePlainText, Data: utils.BytesToString(text),
			})
		case img, ok := <-imagCh:
			if !ok {
				return
			}
			utils.Request(http.MethodPost, url, &types.ClipboardData{
				Type: types.ClipboardDataTypeImagePNG,
				Data: base64.StdEncoding.EncodeToString(img),
			})
		}
	}
}
