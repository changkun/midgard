// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package wire is what a device and the server say to each other over the
// websocket (specs/redesign.md §6, §8).
//
// Every message is one binary frame: an envelope, as one line of JSON, then
// the bytes of the copy it carries, if any, one format after another. The
// server reads envelopes and never the bytes, which it only passes on; that
// is what lets the bytes become ciphertext later (§11) without the server
// changing.
package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Version is the protocol a device speaks, sent in its hello.
const Version = 1

// MaxPayload bounds the bytes a frame may carry: a copy of at most 32 MB,
// the size of the largest share.
const MaxPayload = 32 << 20

// Type is what a frame is for.
type Type string

// The frames. A device says hello once, then sends what happens on it and
// acknowledges what it applied; the server numbers what happens and sends it
// to each of the person's devices, and asks devices for what it does not
// hold.
const (
	Hello   Type = "hello"   // device: who it is, and what it has
	Welcome Type = "welcome" // server: the answer to hello
	Copy    Type = "copy"    // device: a copy made on it
	Delete  Type = "delete"  // device: remove a copy from the history
	Clear   Type = "clear"   // device: remove every copy numbered so far
	Event   Type = "event"   // server: an event of the history, numbered
	Ack     Type = "ack"     // device: it has applied everything it was sent up to Seq
	Want    Type = "want"    // either: send me these events
	Have    Type = "have"    // device: one event it was asked for
	Done    Type = "done"    // device: the end of an answer to want
)

// Kind is what an event of the history is.
type Kind string

// The kinds of event. A void is an event that holds nothing: a copy taken
// back before any device had it, or one deleted while the server still held
// it. Devices keep it as a numbered gap that is not missing.
const (
	KindCopy   Kind = "copy"
	KindDelete Kind = "delete" // Target is the seq of the copy
	KindClear  Kind = "clear"  // Target is the last seq it clears
	KindVoid   Kind = "void"
)

// Format is one representation of a copy: its type and how many of the
// payload's bytes are it.
type Format struct {
	MIME string `json:"mime"`
	Size int    `json:"size"`
}

// Envelope is what the server reads of a frame. Which fields a frame uses
// depends on its type; the rest are left out.
type Envelope struct {
	Type Type `json:"type"`

	// hello
	V      int    `json:"v,omitempty"`      // Version
	Device string `json:"device,omitempty"` // the install's id
	Name   string `json:"name,omitempty"`   // the device's name, to show
	Clock  int64  `json:"clock,omitempty"`  // the device's clock, unix ms
	Gaps   []Span `json:"gaps,omitempty"`   // what it lacks below Acked

	// welcome
	Head uint64 `json:"head,omitempty"` // the last seq given out

	// hello and ack: the highest seq the device has applied
	Acked uint64 `json:"acked,omitempty"`

	// events, copies, deletes and clears
	Seq     uint64   `json:"seq,omitempty"`
	Kind    Kind     `json:"kind,omitempty"`
	Time    int64    `json:"time,omitempty"`    // when it happened, unix ms
	Origin  string   `json:"origin,omitempty"`  // which device, or "web", or a token's name
	Ref     string   `json:"ref,omitempty"`     // the device's own name for what it sent, echoed back
	Offline bool     `json:"offline,omitempty"` // made while the device was not connected
	Target  uint64   `json:"target,omitempty"`  // delete and clear
	Formats []Format `json:"formats,omitempty"`

	// want, have and done
	ID      string `json:"id,omitempty"`      // pairs an answer with its want
	Span    *Span  `json:"span,omitempty"`    // want: these events, by seq
	Newest  bool   `json:"newest,omitempty"`  // want: the newest copy
	List    int    `json:"list,omitempty"`    // want: the newest n copies, without their bytes
	Preview int    `json:"preview,omitempty"` // want, with List: the first n bytes of each text, as its payload
	Bare    bool   `json:"bare,omitempty"`    // have: the bytes are left out, but for a preview
	Err     string `json:"err,omitempty"`     // done: why there is no answer
}

// Span is the events from From to To, both included.
type Span struct {
	From uint64 `json:"from"`
	To   uint64 `json:"to"`
}

// Frame is an envelope and the bytes it carries.
type Frame struct {
	Envelope
	Payload []byte
}

// Size is the number of bytes the formats say the payload holds.
func (e *Envelope) Size() int {
	n := 0
	for _, f := range e.Formats {
		n += f.Size
	}
	return n
}

// At is the event's time.
func (e *Envelope) At() time.Time { return time.UnixMilli(e.Time) }

// Marshal encodes f as one websocket message.
func (f *Frame) Marshal() ([]byte, error) {
	if !f.Bare && f.Size() != len(f.Payload) {
		return nil, fmt.Errorf("wire: the formats hold %d bytes, the payload %d", f.Size(), len(f.Payload))
	}
	head, err := json.Marshal(&f.Envelope)
	if err != nil {
		return nil, err
	}
	b := make([]byte, 0, len(head)+1+len(f.Payload))
	b = append(b, head...)
	b = append(b, '\n')
	return append(b, f.Payload...), nil
}

// Unmarshal decodes one websocket message.
func Unmarshal(b []byte) (Frame, error) {
	head, payload, ok := bytes.Cut(b, []byte{'\n'})
	if !ok {
		return Frame{}, errors.New("wire: a frame without an envelope")
	}
	var f Frame
	if err := json.Unmarshal(head, &f.Envelope); err != nil {
		return Frame{}, fmt.Errorf("wire: %w", err)
	}
	if f.Type == "" {
		return Frame{}, errors.New("wire: a frame without a type")
	}
	for _, format := range f.Formats {
		if format.Size < 0 {
			return Frame{}, errors.New("wire: a format of negative size")
		}
	}
	if len(payload) > MaxPayload {
		return Frame{}, fmt.Errorf("wire: a payload of %d bytes, over %d", len(payload), MaxPayload)
	}
	if !f.Bare && f.Size() != len(payload) {
		return Frame{}, fmt.Errorf("wire: the formats hold %d bytes, the payload %d", f.Size(), len(payload))
	}
	if len(payload) > 0 {
		f.Payload = payload
	}
	return f, nil
}

// Parts splits the payload into the bytes of each format, in order.
func (f *Frame) Parts() [][]byte {
	parts := make([][]byte, 0, len(f.Formats))
	rest := f.Payload
	for _, format := range f.Formats {
		if format.Size > len(rest) {
			break
		}
		parts = append(parts, rest[:format.Size])
		rest = rest[format.Size:]
	}
	return parts
}

// Part returns the bytes of the first format of type mime.
func (f *Frame) Part(mime string) ([]byte, bool) {
	for i, p := range f.Parts() {
		if f.Formats[i].MIME == mime {
			return p, true
		}
	}
	return nil, false
}

// NewCopy is a copy of one format.
func NewCopy(mime string, data []byte) Frame {
	return Frame{
		Envelope: Envelope{Type: Copy, Kind: KindCopy, Formats: []Format{{MIME: mime, Size: len(data)}}},
		Payload:  data,
	}
}
