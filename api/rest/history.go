// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"cmp"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"unicode/utf8"

	"changkun.de/x/midgard/internal/history"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/wire"
	"github.com/gin-gonic/gin"
)

// The history is on the person's devices (specs/redesign.md §6): the server
// asks one of them for it, and numbers deletes and clears like copies, so
// they reach every device in the same order.

// History lists the requester's copies, newest first, without their
// content, which GET /history/{id} returns one at a time.
func (m *Midgard) History(c *gin.Context) {
	ctx := c.Request.Context()
	rm, err := m.rel().room(ctx, c.GetString(ctxOwner))
	if err != nil {
		readFailed(c, err)
		return
	}
	if sealedRefused(c, rm) {
		return
	}
	answers, err := m.rel().query(ctx, rm, wire.Envelope{List: history.MaxCopies, Preview: previewLen})
	if err != nil {
		readFailed(c, err)
		return
	}
	out := types.HistoryOutput{History: []types.HistoryEntry{}}
	for _, f := range answers {
		if f.Kind != wire.KindCopy || len(f.Formats) == 0 {
			continue
		}
		e := types.HistoryEntry{
			ID: int64(f.Seq), Device: f.Origin, Created: f.At().UTC(),
			Type: types.MIME(f.Formats[0].MIME), Size: f.Size(),
		}
		if f.Kid != "" {
			// the device cut it at a character before it sealed it (§11)
			e.Kid, e.Preview = f.Kid, base64.StdEncoding.EncodeToString(f.Payload)
		} else {
			e.Preview = validPrefix(f.Payload)
		}
		out.History = append(out.History, e)
	}
	c.JSON(http.StatusOK, out)
}

// previewLen is how much of each text the history shows.
const previewLen = 240

// validPrefix is b as text, without a character a preview cut in two.
func validPrefix(b []byte) string {
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return string(b)
}

// HistoryEntry returns one copy from the requester's history, encoded as
// GET /clipboard encodes the clipboard: from what the relay holds, or from a
// device.
func (m *Midgard) HistoryEntry(c *gin.Context) {
	seq, ok := seqParam(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	rm, err := m.rel().room(ctx, c.GetString(ctxOwner))
	if err != nil {
		readFailed(c, err)
		return
	}
	if sealedRefused(c, rm) {
		return
	}
	if f, ok := rm.copyHeld(seq); ok {
		c.JSON(http.StatusOK, clipboardData(f))
		return
	}
	answers, err := m.rel().query(ctx, rm, wire.Envelope{Span: &wire.Span{From: seq, To: seq}})
	if err != nil {
		readFailed(c, err)
		return
	}
	for _, f := range answers {
		if f.Seq == seq && f.Kind == wire.KindCopy {
			c.JSON(http.StatusOK, clipboardData(f))
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"msg": "no such copy in your history"})
}

// DeleteHistoryEntry removes one copy from the requester's history, on every
// device. It does not change what is on anyone's clipboard.
func (m *Midgard) DeleteHistoryEntry(c *gin.Context) {
	seq, ok := seqParam(c)
	if !ok {
		return
	}
	m.remove(c, wire.Frame{Envelope: wire.Envelope{Type: wire.Delete, Kind: wire.KindDelete, Target: seq}}, seq)
}

// ClearHistory removes every copy numbered so far from the requester's
// history, on every device.
func (m *Midgard) ClearHistory(c *gin.Context) {
	m.remove(c, wire.Frame{Envelope: wire.Envelope{Type: wire.Clear, Kind: wire.KindClear}}, 0)
}

func (m *Midgard) remove(c *gin.Context, f wire.Frame, seq uint64) {
	ctx := c.Request.Context()
	owner := c.GetString(ctxOwner)
	head, err := m.rel().store.Head(ctx, owner)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	if seq > head {
		c.JSON(http.StatusNotFound, gin.H{"msg": "no such copy in your history"})
		return
	}
	rm, err := m.rel().room(ctx, owner)
	if err == nil {
		_, err = m.rel().number(ctx, rm, nil, cmp.Or(c.GetString(ctxDevice), "web"), f)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// seqParam is the copy's number in the path, or answers 404.
func seqParam(c *gin.Context) (uint64, bool) {
	seq, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || seq == 0 {
		c.JSON(http.StatusNotFound, gin.H{"msg": "no such copy in your history"})
		return 0, false
	}
	return seq, true
}

// Queue is what the server holds for the requester until each of their
// devices has it, and the numbers to tell a delivered copy from a lost one.
func (m *Midgard) Queue(c *gin.Context) {
	out, err := m.rel().queue(c.Request.Context(), c.GetString(ctxOwner))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

// TakeBack takes back a copy none of the requester's devices has yet.
func (m *Midgard) TakeBack(c *gin.Context) {
	seq, ok := seqParam(c)
	if !ok {
		return
	}
	rm, err := m.rel().room(c.Request.Context(), c.GetString(ctxOwner))
	if err == nil {
		err = m.rel().takeBack(rm, seq)
	}
	switch {
	case errors.Is(err, errNotHeld):
		c.JSON(http.StatusNotFound, gin.H{"msg": err.Error()})
	case errors.Is(err, errArrived):
		c.JSON(http.StatusConflict, gin.H{"msg": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
	default:
		c.Status(http.StatusNoContent)
	}
}

// ForgetDevice forgets one of the requester's devices: the server stops
// holding copies for it, until it connects again.
func (m *Midgard) ForgetDevice(c *gin.Context) {
	rm, err := m.rel().room(c.Request.Context(), c.GetString(ctxOwner))
	if err == nil {
		err = m.rel().forget(c.Request.Context(), rm, c.Param("id"))
	}
	switch {
	case errors.Is(err, errNoSuchID):
		c.JSON(http.StatusNotFound, gin.H{"msg": "you have no such device"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
	default:
		c.Status(http.StatusNoContent)
	}
}
