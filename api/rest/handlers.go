// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"cmp"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/version"
	"changkun.de/x/midgard/internal/wire"
	"github.com/gin-gonic/gin"
)

// PingPong is a naive handler for health checking
func (m *Midgard) PingPong(c *gin.Context) {
	c.JSON(http.StatusOK, types.PingOutput{
		Version:   version.GitVersion,
		GoVersion: version.GoVersion,
		BuildTime: version.BuildTime,
	})
}

// GetFromUniversalClipboard is the requester's clipboard: the newest copy in
// their history, which is on their devices (specs/redesign.md §8), sealed
// once they have a key (§11).
func (m *Midgard) GetFromUniversalClipboard(c *gin.Context) {
	ctx := c.Request.Context()
	rm, err := m.rel().room(ctx, c.GetString(ctxOwner))
	if err != nil {
		readFailed(c, err)
		return
	}
	if sealedRefused(c, rm) {
		return
	}
	f, ok, err := m.newest(ctx, rm)
	if err != nil {
		readFailed(c, err)
		return
	}
	out := types.GetFromUniversalClipboardOutput{Type: types.MIMEPlainText}
	if ok {
		out = types.GetFromUniversalClipboardOutput(clipboardData(f))
	}
	c.JSON(http.StatusOK, out)
}

// PutToUniversalClipboard copies to the requester's devices: the relay
// numbers it, and holds it until each of their devices has it.
func (m *Midgard) PutToUniversalClipboard(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, wire.MaxPayload/3*4+1<<16)
	var b types.PutToUniversalClipboardInput
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, types.PutToUniversalClipboardOutput{
			Message: fmt.Sprintf("cannot bind requested data, err: %v", err),
		})
		return
	}
	mime := string(cmp.Or(b.Type, types.MIMEPlainText))
	var f wire.Frame
	if b.Kid != "" {
		// sealed by the client (§11): the server sees what it is and its size
		sealed, err := base64.StdEncoding.DecodeString(b.Data)
		if err != nil || len(sealed) <= wire.SealOverhead {
			c.JSON(http.StatusBadRequest, types.PutToUniversalClipboardOutput{Message: "a sealed copy must be its sealed bytes, in base64"})
			return
		}
		f = wire.Frame{
			Envelope: wire.Envelope{Type: wire.Copy, Kind: wire.KindCopy, Kid: b.Kid,
				Formats: []wire.Format{{MIME: mime, Size: len(sealed) - wire.SealOverhead}}},
			Payload: sealed,
		}
	} else {
		raw := []byte(b.Data)
		if b.Type == types.MIMEImagePNG {
			var err error
			if raw, err = base64.StdEncoding.DecodeString(b.Data); err != nil {
				c.JSON(http.StatusBadRequest, types.PutToUniversalClipboardOutput{Message: "an image must be base64"})
				return
			}
		}
		if len(raw) == 0 || string(raw) == "\n" {
			c.JSON(http.StatusBadRequest, types.PutToUniversalClipboardOutput{Message: "nothing to copy"})
			return
		}
		f = wire.NewCopy(mime, raw)
	}
	if len(f.Payload) > wire.MaxPayload {
		c.JSON(http.StatusRequestEntityTooLarge, types.PutToUniversalClipboardOutput{
			Message: fmt.Sprintf("a copy holds at most %d MB", wire.MaxPayload>>20),
		})
		return
	}
	rm, err := m.rel().room(c.Request.Context(), c.GetString(ctxOwner))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.PutToUniversalClipboardOutput{Message: err.Error()})
		return
	}
	origin := cmp.Or(b.DaemonID, c.GetString(ctxDevice), c.ClientIP())
	ev, err := m.rel().number(c.Request.Context(), rm, nil, origin, f)
	if errors.Is(err, errNotSealed) || errors.Is(err, errOtherKey) || errors.Is(err, errNoKey) {
		c.JSON(http.StatusConflict, types.PutToUniversalClipboardOutput{Message: err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.PutToUniversalClipboardOutput{Message: fmt.Sprintf("cannot copy: %v", err)})
		return
	}
	c.JSON(http.StatusOK, types.PutToUniversalClipboardOutput{Message: "copied to your devices", Seq: ev.Seq})
}

// newest is the person's newest copy: what their device online that has the
// most says, or what the relay holds that is newer, which is all there is
// when none is online.
func (m *Midgard) newest(ctx context.Context, rm *room) (wire.Frame, bool, error) {
	best, ok := rm.newest()
	answers, err := m.rel().query(ctx, rm, wire.Envelope{Newest: true})
	switch {
	case errors.Is(err, errNoDevice) && ok:
		return best, true, nil
	case err != nil:
		return wire.Frame{}, false, err
	}
	for _, f := range answers {
		if f.Kind == wire.KindCopy && (!ok || f.Time > best.Time || (f.Time == best.Time && f.Seq > best.Seq)) {
			best, ok = f, true
		}
	}
	return best, ok, nil
}

// clipboardData encodes a copy as GET /clipboard answers: text as it is, an
// image as base64, and sealed bytes as base64 with their key's id, for the
// client to open.
func clipboardData(f wire.Frame) types.ClipboardData {
	if len(f.Formats) == 0 {
		return types.ClipboardData{Type: types.MIMEPlainText}
	}
	out := types.ClipboardData{Type: types.MIME(f.Formats[0].MIME), Device: f.Origin}
	if f.Kid != "" {
		out.Kid, out.Data = f.Kid, base64.StdEncoding.EncodeToString(f.Payload)
		return out
	}
	data := f.Parts()[0]
	if out.Type == types.MIMEImagePNG {
		out.Data = base64.StdEncoding.EncodeToString(data)
	} else {
		out.Data = string(data)
	}
	return out
}

// sealedRefused answers, and reports true, when the person's copies are
// sealed and the client did not say it opens them (§11): it would take
// ciphertext for their clipboard.
func sealedRefused(c *gin.Context, rm *room) bool {
	rm.mu.Lock()
	kid := rm.kid
	rm.mu.Unlock()
	if kid != "" && c.GetHeader(types.HeaderSealed) != "1" {
		c.JSON(http.StatusConflict, gin.H{"msg": "your copies are encrypted: update this client, or pair it"})
		return true
	}
	return false
}

// readFailed answers a read the person's devices could not answer.
func readFailed(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errNoDevice):
		c.JSON(http.StatusServiceUnavailable, gin.H{"msg": "none of your devices is online; your clipboard and history are on them"})
	case errors.Is(err, errNoAnswer):
		c.JSON(http.StatusGatewayTimeout, gin.H{"msg": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
	}
}

// GetPlainClipboard is the newest copy a Shortcuts bridge pushed, in the
// clear, for Get from Midgard (specs/redesign.md §11): the Shortcuts cannot
// open a sealed copy. With no bridge online, there is none.
func (m *Midgard) GetPlainClipboard(c *gin.Context) {
	rm, err := m.rel().room(c.Request.Context(), c.GetString(ctxOwner))
	if err != nil {
		readFailed(c, err)
		return
	}
	rm.mu.Lock()
	f := rm.mirror
	rm.mu.Unlock()
	if f == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"msg": errNoBridge.Error()})
		return
	}
	c.JSON(http.StatusOK, clipboardData(*f))
}

// PutPlainClipboard passes what Send to Midgard sent, in the clear, to a
// Shortcuts bridge, which seals it into the history as a copy of its own.
func (m *Midgard) PutPlainClipboard(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, wire.MaxPayload/3*4+1<<16)
	var b types.PutToUniversalClipboardInput
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, types.PutToUniversalClipboardOutput{Message: fmt.Sprintf("cannot read the request: %v", err)})
		return
	}
	raw := []byte(b.Data)
	if b.Type == types.MIMEImagePNG {
		var err error
		if raw, err = base64.StdEncoding.DecodeString(b.Data); err != nil {
			c.JSON(http.StatusBadRequest, types.PutToUniversalClipboardOutput{Message: "an image must be base64"})
			return
		}
	}
	if len(raw) == 0 || string(raw) == "\n" {
		c.JSON(http.StatusBadRequest, types.PutToUniversalClipboardOutput{Message: "nothing to copy"})
		return
	}
	if len(raw) > wire.MaxPayload {
		c.JSON(http.StatusRequestEntityTooLarge, types.PutToUniversalClipboardOutput{Message: fmt.Sprintf("a copy holds at most %d MB", wire.MaxPayload>>20)})
		return
	}
	rm, err := m.rel().room(c.Request.Context(), c.GetString(ctxOwner))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.PutToUniversalClipboardOutput{Message: err.Error()})
		return
	}
	f := wire.Frame{
		Envelope: wire.Envelope{Type: wire.Plain, Origin: cmp.Or(b.DaemonID, c.GetString(ctxDevice), "Shortcuts"),
			Formats: []wire.Format{{MIME: string(cmp.Or(b.Type, types.MIMEPlainText)), Size: len(raw)}}},
		Payload: raw,
	}
	if err := m.rel().toBridge(rm, f); err != nil {
		c.JSON(http.StatusServiceUnavailable, types.PutToUniversalClipboardOutput{Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, types.PutToUniversalClipboardOutput{Message: "sent to your devices, through your bridge"})
}
