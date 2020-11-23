// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package api

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/renstrom/shortuuid"
	"golang.design/x/midgard/clipboard"
	"golang.design/x/midgard/config"
)

// GetFromUniversalClipboardInput is the standard input format of
// the universal clipboard put request.
type GetFromUniversalClipboardInput struct {
}

// GetFromUniversalClipboardOutput is the standard output format of
// the universal clipboard put request.
type GetFromUniversalClipboardOutput clipboard.Data

// GetFromUniversalClipboard returns the in-memory clipboard data inside
// the midgard server
func GetFromUniversalClipboard(c *gin.Context) {
	t, buf := uc0.read()

	var raw string
	if t == clipboard.DataTypeImagePNG {
		// We stored our clipboard in bytes, if client is retriving this
		// data, then let's encode it into base64.
		raw = base64.StdEncoding.EncodeToString(buf)
	} else {
		raw = string(buf)
	}

	c.JSON(http.StatusOK, GetFromUniversalClipboardOutput{
		Type: t,
		Data: raw,
	})
}

// PutToUniversalClipboardInput is the standard input format of
// the universal clipboard put request.
type PutToUniversalClipboardInput clipboard.Data

// PutToUniversalClipboardOutput is the standard output format of
// the universal clipboard put request.
type PutToUniversalClipboardOutput struct {
	Message string `json:"msg"`
}

// PutToUniversalClipboard saves data to the in-memory clipboard data
// inside the midgrad server.
func PutToUniversalClipboard(c *gin.Context) {
	var b PutToUniversalClipboardInput

	err := c.ShouldBindJSON(&b)
	if err != nil {
		err = fmt.Errorf("cannot bind requested data, err: %w", err)
		c.JSON(http.StatusBadRequest, PutToUniversalClipboardOutput{
			Message: err.Error(),
		})
		return
	}

	var raw []byte
	if b.Type == clipboard.DataTypeImagePNG {
		// We assume the client send us a base64 encoded image data,
		// Let's decode it into bytes.
		raw, err = base64.StdEncoding.DecodeString(b.Data)
		if err != nil {
			raw = []byte{}
		}
	} else {
		raw = []byte(b.Data)
	}

	uc0.put(b.Type, raw)
	c.JSON(http.StatusOK, PutToUniversalClipboardOutput{
		Message: "clipboard data is saved.",
	})
}

// SourceType ...
type SourceType int

const (
	// SourceUniversalClipboard ...
	SourceUniversalClipboard SourceType = iota
	// SourceAttachment ...
	SourceAttachment
)

// URIGeneratorInput defines the input format of requested resource
type URIGeneratorInput struct {
	Source SourceType `json:"source"`
	URI    string     `json:"uri"`
	Data   string     `json:"data"`
}

// URIGeneratorOutput ...
type URIGeneratorOutput struct {
	URL     string `json:"url"`
	Message string `json:"msg"`
}

// URIGenerator generates an universal access URL for the requested resource.
// The requested resource can be an attached data, the midgard universal
// clipboard, and etc.
func URIGenerator(c *gin.Context) {
	var in URIGeneratorInput
	err := c.ShouldBindJSON(&in)
	if err != nil {
		err = fmt.Errorf("cannot bind requested data, err: %w", err)
		c.JSON(http.StatusBadRequest, URIGeneratorOutput{
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
	case SourceUniversalClipboard:
		t, raw := uc0.read()
		data = raw
		fmt.Println("type: ", t)
		if t == clipboard.DataTypeImagePNG {
			ext = ".png"
		}
	case SourceAttachment:
		data = []byte(in.Data)
	}

	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, URIGeneratorOutput{
			Message: "nothing to persist, no data.",
		})
		return
	}

	root := config.Get().Store.Path
	var path string

	// if URI is empty, then generate a random path
	if in.URI == "" {
		path = root + "/wild/" + shortuuid.New() + ext
	} else {
		path = root + "/" + strings.TrimPrefix(in.URI, "/")
	}

	// check if the path is availiable, if not then throw an error
	existed := func(path string) bool {
		_, err := os.Stat(path)
		return !os.IsNotExist(err)
	}

	if existed(path) {
		c.JSON(http.StatusBadRequest, URIGeneratorOutput{
			Message: "the requested uri already existed.",
		})
		return
	}

	dir, _ := filepath.Split(path)
	if !existed(dir) {
		err = os.MkdirAll(dir, os.ModeDir|os.ModePerm)
		if err != nil {
			err = fmt.Errorf("failed to create uri, err: %w", err)
			c.JSON(http.StatusInternalServerError, URIGeneratorOutput{
				Message: err.Error(),
			})
			return
		}
	}

	// everything seems fine, save the data
	err = ioutil.WriteFile(path, data, os.ModePerm)
	if err != nil {
		err = fmt.Errorf("failed to persist the data, err: %w", err)
		c.JSON(http.StatusInternalServerError, URIGeneratorOutput{
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, URIGeneratorOutput{
		URL:     "/midgard" + config.Get().Store.Prefix + strings.TrimPrefix(path, root),
		Message: "success.",
	})
}
