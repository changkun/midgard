// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package watch

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"

	"golang.design/x/midgard/pkg/clipboard"
	"golang.design/x/midgard/pkg/types"
	"golang.design/x/midgard/pkg/utils"
)

// Clipboard listen to the clipboard for a given data
func Clipboard() {
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

			_, err := utils.Request(http.MethodPost, types.ClipboardEndpoint, &types.ClipboardData{
				Type: types.ClipboardDataTypePlainText, Data: utils.BytesToString(text),
			})
			if err != nil {
				log.Printf("failed to sync clipboard, err: %v", err)
			}
		case img, ok := <-imagCh:
			if !ok {
				return
			}
			_, err := utils.Request(http.MethodPost, types.ClipboardEndpoint, &types.ClipboardData{
				Type: types.ClipboardDataTypeImagePNG,
				Data: base64.StdEncoding.EncodeToString(img),
			})
			if err != nil {
				log.Printf("failed to sync clipboard, err: %v", err)
			}
		}
	}
}
