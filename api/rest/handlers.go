// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"cmp"
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
// their history, which is on their devices (specs/redesign.md §8).
func (m *Midgard) GetFromUniversalClipboard(c *gin.Context) {
	f, ok, err := m.newest(c)
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
	ev, err := m.rel().number(c.Request.Context(), rm, nil, origin, wire.NewCopy(string(cmp.Or(b.Type, types.MIMEPlainText)), raw))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.PutToUniversalClipboardOutput{Message: fmt.Sprintf("cannot copy: %v", err)})
		return
	}
	c.JSON(http.StatusOK, types.PutToUniversalClipboardOutput{Message: "copied to your devices", Seq: ev.Seq})
}

// newest is the requester's newest copy: what their device online that has
// the most says, or what the relay holds that is newer, which is all there
// is when none is online.
func (m *Midgard) newest(c *gin.Context) (wire.Frame, bool, error) {
	ctx := c.Request.Context()
	rm, err := m.rel().room(ctx, c.GetString(ctxOwner))
	if err != nil {
		return wire.Frame{}, false, err
	}
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
// image as base64.
func clipboardData(f wire.Frame) types.ClipboardData {
	if len(f.Formats) == 0 {
		return types.ClipboardData{Type: types.MIMEPlainText}
	}
	t := types.MIME(f.Formats[0].MIME)
	data := f.Parts()[0]
	if t == types.MIMEImagePNG {
		return types.ClipboardData{Type: t, Data: base64.StdEncoding.EncodeToString(data)}
	}
	return types.ClipboardData{Type: t, Data: string(data)}
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
