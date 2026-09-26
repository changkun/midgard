// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"

	"changkun.de/x/midgard/internal/store"
	"changkun.de/x/midgard/internal/types"
	"github.com/gin-gonic/gin"
)

// History lists the requester's copies, newest first, without their
// content, which GET /history/{id} returns one at a time.
func (m *Midgard) History(c *gin.Context) {
	clips, err := m.store.History(c.Request.Context(), c.GetString(ctxOwner))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	out := types.HistoryOutput{History: []types.HistoryEntry{}}
	for _, clip := range clips {
		out.History = append(out.History, types.HistoryEntry{
			ID: clip.ID, Device: clip.Device, Created: clip.Created,
			Type: types.MIME(clip.MIME), Size: clip.Size,
		})
	}
	c.JSON(http.StatusOK, out)
}

// HistoryEntry returns one copy from the requester's history, encoded as
// GET /clipboard encodes the clipboard. An id from someone else's history is
// not found, the same as one that never was.
func (m *Midgard) HistoryEntry(c *gin.Context) {
	id, ok := clipID(c)
	if !ok {
		return
	}
	clip, err := m.store.Clip(c.Request.Context(), c.GetString(ctxOwner), id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"msg": "no such copy in your history"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	data := string(clip.Data)
	if clip.MIME == string(types.MIMEImagePNG) {
		data = base64.StdEncoding.EncodeToString(clip.Data)
	}
	c.JSON(http.StatusOK, types.ClipboardData{Type: types.MIME(clip.MIME), Data: data})
}

// DeleteHistoryEntry removes one copy from the requester's history.
func (m *Midgard) DeleteHistoryEntry(c *gin.Context) {
	id, ok := clipID(c)
	if !ok {
		return
	}
	err := m.store.DeleteClip(c.Request.Context(), c.GetString(ctxOwner), id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"msg": "no such copy in your history"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ClearHistory removes all of the requester's history.
func (m *Midgard) ClearHistory(c *gin.Context) {
	if err := m.store.ClearHistory(c.Request.Context(), c.GetString(ctxOwner)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func clipID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "a history entry is named by its number"})
		return 0, false
	}
	return id, true
}
