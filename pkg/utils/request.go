// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"golang.design/x/midgard/config"
	"golang.design/x/midgard/pkg/types/proto"
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
	// TODO: authentication.
	conn, err := grpc.Dial(config.D().ServerAddr.RPC, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: \n\t%v", err)
	}
	defer conn.Close()
	client := proto.NewMidgardClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	callback(ctx, client)
}
