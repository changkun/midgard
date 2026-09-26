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
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/signin"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
)

// Share publishes data at a link and returns it. With no data it publishes
// this person's clipboard instead. A name, when given, is a link of its own
// besides the random one. filename, the source file's name if there is one,
// gives the share its type and the name its extension. A positive expires
// retires the share after that long.
func Share(name string, data []byte, filename string, expires time.Duration) (types.ShareInfo, error) {
	in := types.ShareInput{ExpiresIn: int64(expires / time.Second)}
	if len(data) > 0 {
		in.Data = base64.StdEncoding.EncodeToString(data)
		in.Type = types.MIME(cmp.Or(mime.TypeByExtension(path.Ext(filename)), http.DetectContentType(data)))
	}
	in.Name = name
	if ext := path.Ext(filename); name != "" && ext != "" {
		// the link takes the file's extension, so it opens as what it is
		in.Name = strings.TrimSuffix(name, path.Ext(name)) + ext
	}
	var out types.ShareInfo
	if err := call(http.MethodPost, types.EndpointShares(), &in, &out); err != nil {
		return out, err
	}
	out.URL = config.ServerURL() + out.URL
	return out, nil
}

// Shares lists this person's shares, newest first, with full links.
func Shares() ([]types.ShareInfo, error) {
	var out types.SharesOutput
	if err := call(http.MethodGet, types.EndpointShares(), nil, &out); err != nil {
		return nil, err
	}
	for i := range out.Shares {
		out.Shares[i].URL = config.ServerURL() + out.Shares[i].URL
	}
	return out.Shares, nil
}

// DeleteShare revokes this person's share slug; its links stop working.
func DeleteShare(slug string) error {
	return call(http.MethodDelete, types.EndpointShares()+"/"+slug, nil, nil)
}

// Devices lists this person's devices, online or not.
func Devices() ([]types.Device, error) {
	var out types.DevicesOutput
	err := call(http.MethodGet, types.EndpointDevices(), nil, &out)
	return out.Devices, err
}

// ForgetDevice forgets this person's device id: the server stops holding
// copies for it, until it connects again.
func ForgetDevice(id string) error {
	return call(http.MethodDelete, types.EndpointDevices()+"/"+id, nil, nil)
}

// Queue is what the server holds until each of this person's devices has
// it.
func Queue() (types.QueueOutput, error) {
	var out types.QueueOutput
	err := call(http.MethodGet, types.EndpointQueue(), nil, &out)
	return out, err
}

// TakeBack takes back a copy none of this person's devices has yet.
func TakeBack(seq uint64) error {
	return call(http.MethodDelete, types.EndpointQueue()+"/"+strconv.FormatUint(seq, 10), nil, nil)
}

// ErrNotFound means the server has no such thing for this person.
var ErrNotFound = errors.New("not found")

// ErrNoDevice means none of this person's devices is online to answer a read
// of their clipboard or history, which are on the devices.
var ErrNoDevice = errors.New("none of your devices is online")

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
	case status == http.StatusServiceUnavailable:
		return ErrNoDevice
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
	return decode(out)
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
// it to their devices, and returns its number in the history. A command
// cannot keep it there itself: on X11 and Wayland a copy lasts only while the
// program that made it runs, and a command exits at once. The daemons run on.
func Copy(t types.MIME, data []byte) (uint64, error) {
	in := types.PutToUniversalClipboardInput{ClipboardData: types.ClipboardData{Type: t, Data: string(data)}}
	if t == types.MIMEImagePNG {
		in.Data = base64.StdEncoding.EncodeToString(data)
	}
	var out types.PutToUniversalClipboardOutput
	err := call(http.MethodPost, types.EndpointClipboard(), &in, &out)
	return out.Seq, err
}

// Clipboard is this person's clipboard: the newest copy, from one of their
// devices or from what the server holds. ErrNoDevice when there is neither.
func Clipboard() (types.MIME, []byte, error) {
	var out types.ClipboardData
	if err := call(http.MethodGet, types.EndpointClipboard(), nil, &out); err != nil {
		return "", nil, err
	}
	return decode(out)
}

// decode is the bytes of a copy as the API encodes it: an image in base64.
func decode(d types.ClipboardData) (types.MIME, []byte, error) {
	if d.Type == types.MIMEImagePNG {
		b, err := base64.StdEncoding.DecodeString(d.Data)
		return d.Type, b, err
	}
	return d.Type, []byte(d.Data), nil
}
