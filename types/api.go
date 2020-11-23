// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package types

// GetFromUniversalClipboardInput is the standard input format of
// the universal clipboard put request.
type GetFromUniversalClipboardInput struct {
}

// GetFromUniversalClipboardOutput is the standard output format of
// the universal clipboard put request.
type GetFromUniversalClipboardOutput ClipboardData

// PutToUniversalClipboardInput is the standard input format of
// the universal clipboard put request.
type PutToUniversalClipboardInput ClipboardData

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

// URIGeneratorInput defines the input format of requested resource
type URIGeneratorInput struct {
	Source SourceType `json:"source"`
	URI    string     `json:"uri"`
	Data   string     `json:"data"`
}

// URIGeneratorOutput ...
type URIGeneratorOutput struct {
	URL     string `json:"url"`
	Message string `json:"msg"`
}
