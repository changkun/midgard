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
//
// Note: We use string for the data type because this is better
// for post body in iOS shortcut.
type ClipboardDataType string

const (
	// ClipboardDataTypePlainText indicates plain text data type
	ClipboardDataTypePlainText ClipboardDataType = "text"
	// ClipboardDataTypeImagePNG indicates image/png data type
	ClipboardDataTypeImagePNG = "image/png"
)
