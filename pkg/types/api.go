// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package types

import (
	"encoding/json"

	"golang.design/x/midgard/config"
)

// Endpoints
var (
	ClipboardEndpoint   = config.Get().Domain + "/midgard/api/v1/clipboard"
	ClipboardWSEndpoint = ClipboardEndpoint + "/ws"
	AllocateURLEndpoint = config.Get().Domain + "/midgard/api/v1/allocate"
)

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

// SourceType ...
type SourceType int

const (
	// SourceUniversalClipboard ...
	SourceUniversalClipboard SourceType = iota
	// SourceAttachment ...
	SourceAttachment
)

// SubscribeAction represents daemon subscribe action type.
type SubscribeAction string

// All actions from daemons
var (
	ActionNone             SubscribeAction = "none"
	ActionRegister         SubscribeAction = "register"
	ActionReady            SubscribeAction = "ready"
	ActionClipboardChanged SubscribeAction = "cbchanged"
	ActionTerminate        SubscribeAction = "terminate"
)

// SubscribeMessage represents the communication between midgard server
// and midgard daemon.
type SubscribeMessage struct {
	Action   SubscribeAction `json:"action"`
	DaemonID string          `json:"daemon_id"`
	Message  string          `json:"msg"`
}

// Encode encodes a subscribe message
func (sm *SubscribeMessage) Encode() []byte {
	b, _ := json.Marshal(sm)
	return b
}

// AllocateURLInput defines the input format of requested resource
type AllocateURLInput struct {
	Source SourceType `json:"source"`
	URI    string     `json:"uri"`
	Data   string     `json:"data"`
}

// AllocateURLOutput ...
type AllocateURLOutput struct {
	URL     string `json:"url"`
	Message string `json:"msg"`
}
