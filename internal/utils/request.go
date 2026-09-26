// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"changkun.de/x/midgard/internal/signin"
)

// Request conducts a http request for a given method, api endpoint, and
// data attached as application/json Content-Type.
func Request(method, api string, data any) ([]byte, error) {
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

	c := &http.Client{}
	req, err := http.NewRequest(method, api, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	auth, err := signin.Authorization(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
