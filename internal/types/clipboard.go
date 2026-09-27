// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package types

// ClipboardData is a copy: its type and its bytes, as text, or an image in
// base64. Sealed (specs/redesign.md §11), Kid names the key and Data is the
// sealed bytes in base64, whatever the type. Device is the one it came from,
// in an answer.
type ClipboardData struct {
	Type   MIME   `json:"type"`
	Data   string `json:"data"`
	Kid    string `json:"kid,omitempty"`
	Device string `json:"device,omitempty"`
}

// HeaderSealed is the header a client sends to say it opens sealed copies:
// without it, a person's server refuses to hand theirs out, rather than give
// ciphertext to a client that would take it for text (§11).
const HeaderSealed = "Midgard-Sealed"

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
