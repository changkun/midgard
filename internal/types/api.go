// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package types

import (
	"strings"
	"time"

	"changkun.de/x/midgard/internal/config"
)

// Endpoints of the midgard server. They are functions rather than variables so
// that the configuration naming the server is read when a request is made, not
// when the program starts.

// EndpointClipboard is the universal clipboard.
func EndpointClipboard() string { return config.ServerURL() + "/midgard/api/v1/clipboard" }

// EndpointShares publishes and lists shares.
func EndpointShares() string { return config.ServerURL() + "/midgard/api/v1/shares" }

// EndpointDevices lists the daemons connected to the server.
func EndpointDevices() string { return config.ServerURL() + "/midgard/api/v1/devices" }

// EndpointSubscribe is the websocket daemons subscribe to, as a ws:// or
// wss:// URL.
func EndpointSubscribe() string {
	return "ws" + strings.TrimPrefix(config.ServerURL(), "http") + "/midgard/api/v1/ws"
}

// PingInput is the input for /ping
type PingInput struct{}

// PingOutput is the output for /ping
type PingOutput struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	BuildTime string `json:"build_time"`
}

// GetFromUniversalClipboardInput is the standard input format of
// the universal clipboard put request.
type GetFromUniversalClipboardInput struct {
}

// GetFromUniversalClipboardOutput is the standard output format of
// the universal clipboard put request.
type GetFromUniversalClipboardOutput ClipboardData

// PutToUniversalClipboardInput is the standard input format of
// the universal clipboard put request.
type PutToUniversalClipboardInput struct {
	ClipboardData
	DaemonID string `json:"daemon_id"`
}

// PutToUniversalClipboardOutput is the standard output format of
// the universal clipboard put request.
type PutToUniversalClipboardOutput struct {
	Message string `json:"msg"`
}

// ShareInput asks to publish a share: Data, base64, of Type, or the
// requester's clipboard when Data is empty. Name, when set, is a name for its
// link besides the random one; ExpiresIn, in seconds, when set, retires it.
type ShareInput struct {
	Data      string `json:"data,omitempty"`
	Type      MIME   `json:"type,omitempty"`
	Name      string `json:"name,omitempty"`
	ExpiresIn int64  `json:"expires_in,omitempty"`
}

// ShareInfo describes a share. URL is the link to hand out, relative to the
// server: its name when it has one, its random link otherwise.
type ShareInfo struct {
	Slug    string     `json:"slug"`
	Name    string     `json:"name,omitempty"`
	URL     string     `json:"url"`
	Created time.Time  `json:"created"`
	Expires *time.Time `json:"expires,omitempty"`
	Type    MIME       `json:"type"`
	Size    int        `json:"size"`
}

// SharesOutput is the answer to GET /shares, newest first.
type SharesOutput struct {
	Shares []ShareInfo `json:"shares"`
}

// Device is a daemon connected to the server.
type Device struct {
	Index uint64 `json:"index"`
	Name  string `json:"name"`
}

// DevicesOutput is the answer to GET /devices.
type DevicesOutput struct {
	Devices []Device `json:"devices"`
}

// EndpointHistory is a person's clipboard history.
func EndpointHistory() string { return config.ServerURL() + "/midgard/api/v1/history" }

// HistoryEntry is one copy in a person's history, without its content.
type HistoryEntry struct {
	ID      int64     `json:"id"`
	Device  string    `json:"device"`
	Created time.Time `json:"created"`
	Type    MIME      `json:"type"`
	Size    int       `json:"size"`
}

// HistoryOutput is the answer to GET /history, newest first.
type HistoryOutput struct {
	History []HistoryEntry `json:"history"`
}
