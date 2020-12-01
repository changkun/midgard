// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
)

// All actions from daemons
const (
	ActionNone              WebsocketAction = "none"
	ActionHandshakeRegister                 = "register"
	ActionHandshakeReady                    = "ready"
	ActionClipboardChanged                  = "cbchanged"
	ActionClipboardGet                      = "cbget"
	ActionClipboardPut                      = "cbput"
	ActionTerminate                         = "terminate"
)

// WebsocketAction is an action between midgard daemon and midgard server
type WebsocketAction string

// WebsocketMessage represents a message for websocket.
type WebsocketMessage struct {
	Action  WebsocketAction `json:"action"`
	UserID  string          `json:"user_id"`
	Message string          `json:"msg"`
	Data    []byte          `json:"data"` // action dependent data, json format
}

// Encode encodes a websocket message
func (m *WebsocketMessage) Encode() []byte {
	b, _ := json.Marshal(m)
	return b
}

// Decode decodes given data to m.
func (m *WebsocketMessage) Decode(data []byte) error {
	return json.Unmarshal(data, m)
}
