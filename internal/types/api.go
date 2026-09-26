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

// EndpointQueue is what the server holds until every device has it.
func EndpointQueue() string { return config.ServerURL() + "/midgard/api/v1/queue" }

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
	Seq     uint64 `json:"seq,omitempty"` // the copy's number in the history
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

// Device is one of a person's devices: one install of midgard.
type Device struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Online   bool      `json:"online"`
	LastSeen time.Time `json:"last_seen"`
	Acked    uint64    `json:"acked"`   // the highest number it has applied
	Counted  bool      `json:"counted"` // the server holds copies until it has them
}

// DevicesOutput is the answer to GET /devices: one's devices, and the last
// number given out.
type DevicesOutput struct {
	Head    uint64   `json:"head"`
	Devices []Device `json:"devices"`
}

// QueueEntry is something the server holds until every device has it.
type QueueEntry struct {
	Seq     uint64    `json:"seq"`
	Kind    string    `json:"kind"` // copy, delete, clear, or void
	Created time.Time `json:"created"`
	Origin  string    `json:"origin"`
	Type    MIME      `json:"type,omitempty"`
	Size    int       `json:"size"`
	Waiting []string  `json:"waiting"` // the ids of the devices that lack it
}

// QueueOutput is the answer to GET /queue: what is on its way, and to
// which devices. A number no device has and the queue no longer holds was
// lost, as when the server restarted before a device came back.
type QueueOutput struct {
	Head    uint64       `json:"head"`
	Devices []Device     `json:"devices"`
	Queue   []QueueEntry `json:"queue"`
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

// TokenInput asks for an app token named Name.
type TokenInput struct {
	Name string `json:"name"`
}

// TokenInfo describes an app token. Token is the token itself, and only in
// the answer that issued it; it is never shown again.
type TokenInfo struct {
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
	Token   string    `json:"token,omitempty"`
}

// TokensOutput is the answer to GET /tokens.
type TokensOutput struct {
	Tokens []TokenInfo `json:"tokens"`
}
