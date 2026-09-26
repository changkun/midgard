// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package client is how mg commands talk to the midgard server: directly,
// over HTTP, with the device's credentials. They used to go through the local
// daemon over gRPC, which only relayed each call to the server.
package client

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/signin"
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
	if errors.Is(err, signin.ErrSignedOut) {
		return "", err // not a network problem; say what to do
	}
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
	if errors.Is(err, signin.ErrSignedOut) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("cannot reach the midgard server: %w", err)
	}
	var out types.DevicesOutput
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, fmt.Errorf("cannot parse the server's answer: %w", err)
	}
	return out.Devices, nil
}

// ErrNotFound means the server has no such thing for this person.
var ErrNotFound = errors.New("not found")

// call makes a request and turns a failure into an error, with the server's
// reason when it gives one.
func call(method, api string, in, out any) error {
	status, body, err := utils.Do(method, api, in)
	if errors.Is(err, signin.ErrSignedOut) {
		return err
	}
	if err != nil {
		return fmt.Errorf("cannot reach the midgard server: %w", err)
	}
	switch {
	case status == http.StatusNotFound:
		return ErrNotFound
	case status >= 300:
		var reason struct {
			Msg string `json:"msg"`
		}
		json.Unmarshal(body, &reason)
		return fmt.Errorf("the server answered %d: %s", status, cmp.Or(reason.Msg, http.StatusText(status)))
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}

// History lists this person's clipboard history, newest first.
func History() ([]types.HistoryEntry, error) {
	var out types.HistoryOutput
	err := call(http.MethodGet, types.EndpointHistory(), nil, &out)
	return out.History, err
}

// HistoryEntry returns the copy numbered id from this person's history.
func HistoryEntry(id int64) (types.MIME, []byte, error) {
	var out types.ClipboardData
	if err := call(http.MethodGet, fmt.Sprintf("%s/%d", types.EndpointHistory(), id), nil, &out); err != nil {
		return "", nil, err
	}
	if out.Type == types.MIMEImagePNG {
		b, err := base64.StdEncoding.DecodeString(out.Data)
		return out.Type, b, err
	}
	return out.Type, []byte(out.Data), nil
}

// DeleteHistoryEntry removes the copy numbered id from this person's history.
func DeleteHistoryEntry(id int64) error {
	return call(http.MethodDelete, fmt.Sprintf("%s/%d", types.EndpointHistory(), id), nil, nil)
}

// ClearHistory removes all of this person's history.
func ClearHistory() error {
	return call(http.MethodDelete, types.EndpointHistory(), nil, nil)
}

// Copy puts data on this person's clipboard through the server, which hands
// it to their daemons. A command cannot keep it there itself: on X11 and
// Wayland a copy lasts only while the program that made it runs, and a
// command exits at once. The daemons run on.
func Copy(t types.MIME, data []byte) error {
	in := types.PutToUniversalClipboardInput{ClipboardData: types.ClipboardData{Type: t, Data: string(data)}}
	if t == types.MIMEImagePNG {
		in.Data = base64.StdEncoding.EncodeToString(data)
	}
	return call(http.MethodPost, types.EndpointClipboard(), &in, nil)
}
