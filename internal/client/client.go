// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package client is how mg commands talk to the midgard server: directly,
// over HTTP, with the device's credentials. They used to go through the local
// daemon over gRPC, which only relayed each call to the server.
package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
)

// Allocate publishes data at desired, a path on the server, and returns its
// public URL. With no data it publishes the universal clipboard instead. With
// no desired path the server picks a random one. name, the source file's name
// if there is one, gives the published path its extension.
func Allocate(desired string, data []byte, name string) (string, error) {
	in := types.AllocateURLInput{Source: types.SourceUniversalClipboard}
	if len(data) > 0 {
		in.Source = types.SourceAttachment
		in.Data = base64.StdEncoding.EncodeToString(data)
	}
	if desired != "" {
		// the published path takes the source's extension
		in.URI = strings.TrimSuffix(desired, filepath.Ext(desired)) + filepath.Ext(name)
	}

	res, err := utils.Request(http.MethodPut, types.EndpointAllocateURL(), &in)
	if err != nil {
		return "", fmt.Errorf("cannot reach the midgard server: %w", err)
	}
	var out types.AllocateURLOutput
	if err := json.Unmarshal(res, &out); err != nil {
		return "", fmt.Errorf("cannot parse the server's answer: %w", err)
	}
	if out.URL == "" {
		return "", fmt.Errorf("%s", out.Message)
	}
	return config.ServerURL() + out.URL, nil
}

// Devices lists the daemons connected to the server.
func Devices() ([]types.Device, error) {
	res, err := utils.Request(http.MethodGet, types.EndpointDevices(), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot reach the midgard server: %w", err)
	}
	var out types.DevicesOutput
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, fmt.Errorf("cannot parse the server's answer: %w", err)
	}
	return out.Devices, nil
}
