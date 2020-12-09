// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package types

// ClipboardData is a clipboard data
type ClipboardData struct {
	Type MIME   `json:"type"`
	Data string `json:"data"` // base64 encode if type is an image data
}

// MIME indicates clipboard data type
//
// Note: We use string for the data type because this is better
// for post body in iOS shortcut.
type MIME string

const (
	// MIMEPlainText indicates plain text data type
	MIMEPlainText MIME = "text"
	// MIMEImagePNG indicates image/png data type
	MIMEImagePNG = "image/png"
)
