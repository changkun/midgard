// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package types

import (
	"changkun.de/x/midgard/internal/config"
)

// Endpoints of the midgard server. They are functions rather than variables so
// that the configuration naming the server is read when a request is made, not
// when the program starts.

// EndpointClipboard is the universal clipboard.
func EndpointClipboard() string { return config.Get().Domain + "/midgard/api/v1/clipboard" }

// EndpointAllocateURL allocates a public URL.
func EndpointAllocateURL() string { return config.Get().Domain + "/midgard/api/v1/allocate" }

// EndpointCode2Image renders code as an image.
func EndpointCode2Image() string { return config.Get().Domain + "/midgard/api/v1/code2img" }

// EndpointSubscribe is the websocket daemons subscribe to.
func EndpointSubscribe() string { return config.Get().Domain + "/midgard/api/v1/ws" }

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

// SourceType is the source type for URL allocation.
//
// Note: We use string for the data type because this is better
// for post body in iOS shortcut.
type SourceType string

const (
	// SourceUniversalClipboard indicates source from clipboard
	SourceUniversalClipboard SourceType = "clipboard"
	// SourceAttachment indicates source from attachment
	SourceAttachment = "attachment"
)

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

// Code2ImgInput ...
type Code2ImgInput struct {
	Code string `json:"code"`
}

// Code2ImgOutput ...
type Code2ImgOutput struct {
	Code    string `json:"code"`
	Image   string `json:"img"`
	Message string `json:"msg"`
}
