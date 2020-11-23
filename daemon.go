// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"golang.design/x/midgard/clipboard"
	"golang.design/x/midgard/config"
)

func runDaemon() {
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
	clipboard.Watch(context.Background(), clipboard.DataTypePlainText, textCh)
	imagCh := make(chan []byte, 1)
	clipboard.Watch(context.Background(), clipboard.DataTypeImagePNG, imagCh)
	url := "http://" + config.Get().Addr.HTTP + "/midgard/api/v1/clipboard"
	for {
		select {
		case text, ok := <-textCh:
			if !ok {
				return
			}
			request(http.MethodPost, url, &clipboard.Data{
				Type: clipboard.DataTypePlainText, Data: string(text),
			})
		case img, ok := <-imagCh:
			if !ok {
				return
			}
			request(http.MethodPost, url, &clipboard.Data{
				Type: clipboard.DataTypeImagePNG, Data: base64.StdEncoding.EncodeToString(img),
			})
		}
	}
}

func request(method, api string, data interface{}) ([]byte, error) {
	var (
		body []byte
		err  error
	)
	if data != nil {
		body, err = json.Marshal(data)
	}

	c := &http.Client{}
	req, err := http.NewRequest(method, api, bytes.NewBuffer(body))
	req.SetBasicAuth(config.Get().Auth.User, config.Get().Auth.Pass)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
