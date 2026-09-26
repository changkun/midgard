// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"changkun.de/x/midgard/internal/store"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"changkun.de/x/midgard/internal/version"
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

// GetFromUniversalClipboard returns the in-memory clipboard data inside
// the midgard server
func (m *Midgard) GetFromUniversalClipboard(c *gin.Context) {
	t, buf, err := m.clipboard(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.GetFromUniversalClipboardOutput{})
		return
	}

	var raw string
	if t == types.MIMEImagePNG {
		// We stored our clipboard in bytes, if client is retriving this
		// data, then let's encode it into base64.
		raw = base64.StdEncoding.EncodeToString(buf)
	} else {
		raw = utils.BytesToString(buf)
	}

	c.JSON(http.StatusOK, types.GetFromUniversalClipboardOutput{
		Type: t,
		Data: raw,
	})
}

// PutToUniversalClipboard saves data to the in-memory clipboard data
// inside the midgrad server.
func (m *Midgard) PutToUniversalClipboard(c *gin.Context) {
	var b types.PutToUniversalClipboardInput

	err := c.ShouldBindJSON(&b)
	if err != nil {
		err = fmt.Errorf("cannot bind requested data, err: %w", err)
		c.JSON(http.StatusBadRequest, types.PutToUniversalClipboardOutput{
			Message: err.Error(),
		})
		return
	}

	var raw []byte
	if b.Type == types.MIMEImagePNG {
		// We assume the client send us a base64 encoded image data,
		// Let's decode it into bytes.
		raw, err = base64.StdEncoding.DecodeString(b.Data)
		if err != nil {
			raw = []byte{}
		}
	} else {
		raw = utils.StringToBytes(b.Data)
	}

	if b.DaemonID == "" {
		b.DaemonID = cmp.Or(c.GetString(ctxDevice), c.ClientIP())
	}
	owner := c.GetString(ctxOwner)
	updated, err := m.store.AddClip(c.Request.Context(), owner, b.DaemonID, string(b.Type), raw)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.PutToUniversalClipboardOutput{
			Message: fmt.Sprintf("cannot keep the copy: %v", err),
		})
		return
	}
	c.JSON(http.StatusOK, types.PutToUniversalClipboardOutput{
		Message: "clipboard data is saved.",
	})
	if !updated {
		return
	}

	// Include MIME type information so that the clipboard is
	// consistent after sync propagation.
	raw, _ = json.Marshal(b.ClipboardData)
	m.boardcastMessage(owner, &types.WebsocketMessage{
		Action:  types.ActionClipboardChanged,
		UserID:  b.DaemonID,
		Message: "universal clipboard has changes",
		Data:    raw,
	})
}

// clipboard is the requester's clipboard: the newest copy in their history,
// or an empty text when they have none yet.
func (m *Midgard) clipboard(c *gin.Context) (types.MIME, []byte, error) {
	clip, err := m.store.LatestClip(c.Request.Context(), c.GetString(ctxOwner))
	if errors.Is(err, store.ErrNotFound) {
		return types.MIMEPlainText, []byte{}, nil
	}
	if err != nil {
		return "", nil, err
	}
	return types.MIME(clip.MIME), clip.Data, nil
}
