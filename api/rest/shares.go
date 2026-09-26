// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"cmp"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/store"
	"changkun.de/x/midgard/internal/types"
	"github.com/gin-gonic/gin"
)

// maxShareBytes bounds what one share may hold.
const maxShareBytes = 32 << 20

// CreateShare publishes a file, or the requester's clipboard, at a link.
func (m *Midgard) CreateShare(c *gin.Context) {
	var in types.ShareInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("cannot read the request: %v", err)})
		return
	}
	owner := c.GetString(ctxOwner)

	var (
		mime string
		data []byte
	)
	if in.Data == "" {
		t, raw, err := m.clipboard(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
			return
		}
		mime, data = string(t), raw
	} else {
		raw, err := base64.StdEncoding.DecodeString(in.Data)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "data must be base64"})
			return
		}
		mime, data = cmp.Or(string(in.Type), "application/octet-stream"), raw
	}
	if len(data) == 0 || string(data) == "\n" {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "nothing to share"})
		return
	}
	if len(data) > maxShareBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"msg": fmt.Sprintf("a share holds at most %d MB", maxShareBytes>>20)})
		return
	}

	name := ""
	if in.Name != "" {
		var err error
		if name, err = shareName(in.Name); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
			return
		}
	}
	var expires time.Time
	if in.ExpiresIn > 0 {
		expires = time.Now().Add(time.Duration(in.ExpiresIn) * time.Second)
	}

	sh, err := m.store.CreateShare(c.Request.Context(), owner, name, mime, data, expires)
	if errors.Is(err, store.ErrTaken) {
		c.JSON(http.StatusConflict, gin.H{"msg": fmt.Sprintf("%s is taken; pick another name", in.Name)})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, shareInfo(sh))
}

// Shares lists the requester's shares, newest first.
func (m *Midgard) Shares(c *gin.Context) {
	shares, err := m.store.Shares(c.Request.Context(), c.GetString(ctxOwner))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	out := types.SharesOutput{Shares: []types.ShareInfo{}}
	for _, sh := range shares {
		out.Shares = append(out.Shares, shareInfo(sh))
	}
	c.JSON(http.StatusOK, out)
}

// DeleteShare revokes one of the requester's shares; its links stop working.
func (m *Midgard) DeleteShare(c *gin.Context) {
	err := m.store.DeleteShare(c.Request.Context(), c.GetString(ctxOwner), c.Param("slug"))
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"msg": "you have no such share"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// shareInfo describes sh, with the link to hand out: its name if it has one.
func shareInfo(sh store.Share) types.ShareInfo {
	prefix := config.S().Store.Prefix
	info := types.ShareInfo{
		Slug: sh.Slug, Name: sh.Path, Created: sh.Created, Type: types.MIME(sh.MIME), Size: sh.Size,
		URL: prefix + "/s/" + sh.Slug,
	}
	if sh.Path != "" {
		info.URL = prefix + "/" + sh.Path
	}
	if !sh.Expires.IsZero() {
		info.Expires = &sh.Expires
	}
	return info
}

// serveShare answers the public link of a share: /midgard/s/<slug>, or
// /midgard/<name>.
func (m *Midgard) serveShare(c *gin.Context) {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.Status(http.StatusNotFound)
		return
	}
	prefix := config.S().Store.Prefix + "/"
	rest, ok := strings.CutPrefix(c.Request.URL.Path, prefix)
	if !ok || m.store == nil {
		c.Status(http.StatusNotFound)
		return
	}
	ctx := c.Request.Context()
	var (
		sh  store.Share
		err error
	)
	if slug, ok := strings.CutPrefix(rest, "s/"); ok && !strings.Contains(slug, "/") {
		sh, err = m.store.ShareBySlug(ctx, slug)
	} else {
		sh, err = m.store.ShareByPath(ctx, rest)
	}
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// A share is served from this site's own origin, so a shared page must
	// not run as it: sandbox gives it an origin of its own, where scripts
	// reach no one's cookies, and nosniff keeps a browser to the type given.
	h := c.Writer.Header()
	h.Set("Content-Security-Policy", "sandbox")
	h.Set("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, contentType(sh.MIME), sh.Data)
}

// contentType is what a share is served as. midgard calls text "text".
func contentType(mime string) string {
	switch mime {
	case string(types.MIMEPlainText), "text/plain":
		return "text/plain; charset=utf-8"
	}
	return mime
}

// shareName turns a requested name into one a share may take, or says why it
// cannot. It is part of a URL under the prefix, so it must be a clean
// relative path, name no hidden file, and not start where midgard's own
// routes are.
func shareName(name string) (string, error) {
	p := path.Clean(strings.TrimPrefix(name, "/"))
	if p == "." || !filepath.IsLocal(filepath.FromSlash(p)) {
		return "", fmt.Errorf("invalid name %q", name)
	}
	for seg := range strings.SplitSeq(p, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", fmt.Errorf("invalid name %q: no part of it may start with a dot", name)
		}
	}
	switch first, _, _ := strings.Cut(p, "/"); first {
	case "api", "s", "ping":
		return "", fmt.Errorf("invalid name %q: %s/ is midgard's own", name, first)
	}
	return p, nil
}
