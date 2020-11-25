// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package utils

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"golang.design/x/midgard/config"
)

// Request conducts a http request for a given method, api endpoint, and
// data attached as application/json Content-Type.
func Request(method, api string, data interface{}) ([]byte, error) {
	var (
		body []byte
		err  error
	)
	if data != nil {
		body, err = json.Marshal(data)
	}

	c := &http.Client{}
	req, err := http.NewRequest(method, api, bytes.NewBuffer(body))
	req.SetBasicAuth(config.Get().Server.Auth.User, config.Get().Server.Auth.Pass)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
