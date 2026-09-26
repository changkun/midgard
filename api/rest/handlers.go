// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
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
	t, buf := clipboard.UniversalFor(c.GetString(ctxOwner)).Read()

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

	owner := c.GetString(ctxOwner)
	updated := clipboard.UniversalFor(owner).Write(b.Type, raw)
	c.JSON(http.StatusOK, types.PutToUniversalClipboardOutput{
		Message: "clipboard data is saved.",
	})
	if !updated {
		return
	}

	if b.DaemonID == "" {
		b.DaemonID = c.ClientIP()
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
		t, raw := clipboard.UniversalFor(c.GetString(ctxOwner)).Read()
		data = raw
		if t == types.MIMEImagePNG {
			ext = ".png"
		}
	case types.SourceAttachment:
		data, err = base64.StdEncoding.DecodeString(in.Data)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.AllocateURLOutput{
				Message: fmt.Sprintf("decode error: %v", err),
			})
			return
		}
	}

	if len(data) == 0 || utils.BytesToString(data) == "\n" {
		c.JSON(http.StatusBadRequest, types.AllocateURLOutput{
			Message: "nothing to persist, no data.",
		})
		return
	}

	id, err := utils.NewUUIDShort()
	if err != nil {
		panic(fmt.Errorf("failed to create a short uuid: %v", err))
	}

	// if URI is empty, then generate a random path
	name := "random/" + id + ext
	if in.URI != "" {
		name, err = storeName(in.URI)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.AllocateURLOutput{
				Message: err.Error(),
			})
			return
		}
	}

	err = persist(name, data)
	switch {
	case errors.Is(err, fs.ErrExist):
		c.JSON(http.StatusBadRequest, types.AllocateURLOutput{
			Message: "the requested uri already existed.",
		})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, types.AllocateURLOutput{
			Message: fmt.Sprintf("failed to persist the data, err: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, types.AllocateURLOutput{
		URL:     config.S().Store.Prefix + "/" + name,
		Message: "success.",
	})
}

// storeName turns a requested URI into a slash-separated name inside the
// store, or reports why it cannot be one. The URI comes from the client, so
// it must not climb out of the store ("../"), and it must not name a hidden
// file: a store from before the backup was dropped is a git clone, and .git is
// not something to write into.
func storeName(uri string) (string, error) {
	name := path.Clean(strings.TrimPrefix(uri, "/"))
	if name == "." || !filepath.IsLocal(filepath.FromSlash(name)) {
		return "", fmt.Errorf("invalid uri %q: it must name a file inside the store", uri)
	}
	for seg := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", fmt.Errorf("invalid uri %q: hidden files cannot be allocated", uri)
		}
	}
	return name, nil
}

// persist writes data to name inside the store. It fails with fs.ErrExist
// if the name is taken, rather than replacing what is published there.
//
// The writes go through an os.Root, so neither a crafted name nor a symbolic
// link inside the store can put a file outside it. Files are world-readable
// but not writable, since the store is published as it is.
func persist(name string, data []byte) error {
	if err := os.MkdirAll(config.RepoPath, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(config.RepoPath)
	if err != nil {
		return err
	}
	defer root.Close()

	if dir := path.Dir(name); dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
