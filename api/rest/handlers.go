// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/config"
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
	t, buf := clipboard.Universal.Read()

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

	updated := clipboard.Universal.Write(b.Type, raw)
	c.JSON(http.StatusOK, types.PutToUniversalClipboardOutput{
		Message: "clipboard data is saved.",
	})
	if !updated {
		return
	}

	if b.DaemonID == "" {
		b.DaemonID = c.ClientIP()
	}

	m.boardcastMessage(&types.WebsocketMessage{
		Action:  types.ActionClipboardChanged,
		UserID:  b.DaemonID,
		Message: "universal clipboard has changes",
		Data:    raw, // clipboard data
	})
}

// AllocateURL generates an universal access URL for the requested resource.
// The requested resource can be an attached data, the midgard universal
// clipboard, and etc.
func (m *Midgard) AllocateURL(c *gin.Context) {
	var in types.AllocateURLInput
	err := c.ShouldBindJSON(&in)
	if err != nil {
		err = fmt.Errorf("cannot bind requested data, err: %w", err)
		c.JSON(http.StatusBadRequest, types.AllocateURLOutput{
			Message: err.Error(),
		})
		return
	}

	// check request source, determine resource type.
	// if the type cannot be determined, then mark it as plain text.
	var (
		ext  = ".txt"
		data []byte
	)
	switch in.Source {
	case types.SourceUniversalClipboard:
		t, raw := clipboard.Universal.Read()
		data = raw
		if t == types.MIMEImagePNG {
			ext = ".png"
		}
	case types.SourceAttachment:
		data = utils.StringToBytes(in.Data)
	}

	if len(data) == 0 || utils.BytesToString(data) == "\n" {
		c.JSON(http.StatusBadRequest, types.AllocateURLOutput{
			Message: "nothing to persist, no data.",
		})
		return
	}

	root := config.S().Store.Path
	var path string

	// if URI is empty, then generate a random path
	if in.URI == "" {
		path = root + "/random/" + utils.NewUUID() + ext
	} else {
		path = root + "/" + strings.TrimPrefix(in.URI, "/")
	}

	// check if the path is availiable, if not then throw an error
	existed := func(path string) bool {
		_, err := os.Stat(path)
		return !os.IsNotExist(err)
	}

	if existed(path) {
		c.JSON(http.StatusBadRequest, types.AllocateURLOutput{
			Message: "the requested uri already existed.",
		})
		return
	}

	dir, _ := filepath.Split(path)
	if !existed(dir) {
		err = os.MkdirAll(dir, os.ModeDir|os.ModePerm)
		if err != nil {
			err = fmt.Errorf("failed to create uri, err: %w", err)
			c.JSON(http.StatusInternalServerError, types.AllocateURLOutput{
				Message: err.Error(),
			})
			return
		}
	}

	// everything seems fine, save the data
	err = ioutil.WriteFile(path, data, os.ModePerm)
	if err != nil {
		err = fmt.Errorf("failed to persist the data, err: %w", err)
		c.JSON(http.StatusInternalServerError, types.AllocateURLOutput{
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.AllocateURLOutput{
		URL:     config.S().Store.Prefix + strings.TrimPrefix(path, root),
		Message: "success.",
	})
}
