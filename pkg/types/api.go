// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package types

import (
	"changkun.de/x/midgard/pkg/config"
)

// Endpoints
var (
	EndpointClipboard   = config.Get().Domain + "/midgard/api/v1/clipboard"
	EndpointAllocateURL = config.Get().Domain + "/midgard/api/v1/allocate"
	EndpointCode2Image  = config.Get().Domain + "/midgard/api/v1/code2img"
	EndpointSubscribe   = config.Get().Domain + "/midgard/api/v1/ws"
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
