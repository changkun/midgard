// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"changkun.de/x/midgard/pkg/config"
	"changkun.de/x/midgard/pkg/types/proto"
	"google.golang.org/grpc"
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

	if !strings.HasPrefix(api, "https://") || !strings.HasPrefix(api, "http://") {
		if strings.Contains(config.Get().Domain, "localhost") {
			api = "http://" + api
		} else {
			api = "https://" + api
		}
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

// Connect connects to a midgard client
func Connect(callback func(ctx context.Context, c proto.MidgardClient)) {
	// We don't need authentication here. Daemon is running
	// on a local machine.
	conn, err := grpc.Dial(config.D().Addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: \n\t%v", err)
	}
	defer conn.Close()
	client := proto.NewMidgardClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	callback(ctx, client)
}
