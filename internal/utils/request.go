// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package utils

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"changkun.de/x/midgard/internal/config"
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
		if err != nil {
			return nil, err
		}
	}

	if !strings.HasPrefix(api, "https://") || !strings.HasPrefix(api, "http://") {
		if strings.Contains(config.Get().Domain, "localhost") {
			api = "http://" + api
		} else {
			api = "https://" + api
		}
	}

	c := &http.Client{}
	req, err := http.NewRequest(method, api, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(config.Get().Server.Auth.User, config.Get().Server.Auth.Pass)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
