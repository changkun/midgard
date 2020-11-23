// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package types

// ClipboardData is a clipboard data
type ClipboardData struct {
	Type ClipboardDataType `json:"type"`
	Data string            `json:"data"` // base64 encode if type is an image data
}

// ClipboardDataType indicates clipboard data type
type ClipboardDataType int

const (
	// ClipboardDataTypePlainText indicates plain text data type
	ClipboardDataTypePlainText ClipboardDataType = iota
	// ClipboardDataTypeImagePNG indicates image/png data type
	ClipboardDataTypeImagePNG
)
