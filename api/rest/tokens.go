// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"changkun.de/x/midgard/internal/store"
	"changkun.de/x/midgard/internal/types"
	"github.com/gin-gonic/gin"
)

// Tokens lists the requester's app tokens, without the tokens themselves,
// which the server does not keep.
func (m *Midgard) Tokens(c *gin.Context) {
	tokens, err := m.store.AppTokens(c.Request.Context(), c.GetString(ctxOwner))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	out := types.TokensOutput{Tokens: []types.TokenInfo{}}
	for _, t := range tokens {
		out.Tokens = append(out.Tokens, types.TokenInfo{Name: t.Name, Created: t.Created})
	}
	c.JSON(http.StatusOK, out)
}

// IssueToken issues the requester an app token, for a client that cannot
// sign in, such as an iOS Shortcut. The answer is the only time it is shown.
func (m *Midgard) IssueToken(c *gin.Context) {
	var in types.TokenInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("cannot read the request: %v", err)})
		return
	}
	tok, err := m.store.IssueAppToken(c.Request.Context(), c.GetString(ctxOwner), c.GetString(ctxEmail), in.Name)
	if err != nil {
		// the store says what was wrong with the name
		c.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, types.TokenInfo{Name: in.Name, Created: time.Now().UTC(), Token: tok})
}

// RevokeToken revokes the requester's app token name; the client that has
// it is signed out at once.
func (m *Midgard) RevokeToken(c *gin.Context) {
	err := m.store.RevokeAppToken(c.Request.Context(), c.GetString(ctxOwner), c.Param("name"))
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"msg": "you have no such token"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
