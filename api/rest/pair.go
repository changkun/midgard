// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"changkun.de/x/midgard/internal/types"
	"github.com/gin-gonic/gin"
)

// Pairing gives a new device its person's key (specs/redesign.md §11). A
// paired device leaves the key in a mailbox here, sealed under a pairing code
// the server never sees, and the new device, given the code, takes it. A box
// waits in memory for ten minutes and is taken once: it is useless without
// the code, and a server that restarts only means pairing again.

// The mailboxes' bounds.
const (
	pairTTL = 10 * time.Minute
	pairMax = 8   // boxes waiting for one person, at most
	boxMax  = 256 // bytes of a box: a key, a number, and the seal
)

type pairBox struct {
	box     []byte
	expires time.Time
}

// mailboxes are the boxes waiting, by person, then mailbox.
type mailboxes struct {
	mu    sync.Mutex
	boxes map[string]map[string]pairBox
	now   func() time.Time
}

// leave puts box in owner's mailbox, and reports false when they have too
// many waiting.
func (mb *mailboxes) leave(owner, mailbox string, box []byte) bool {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.sweep()
	if mb.boxes == nil {
		mb.boxes = map[string]map[string]pairBox{}
	}
	mine := mb.boxes[owner]
	if mine == nil {
		mine = map[string]pairBox{}
		mb.boxes[owner] = mine
	}
	if _, ok := mine[mailbox]; !ok && len(mine) >= pairMax {
		return false
	}
	mine[mailbox] = pairBox{box: box, expires: mb.now().Add(pairTTL)}
	return true
}

// take is the box in owner's mailbox, which it no longer holds after.
func (mb *mailboxes) take(owner, mailbox string) ([]byte, bool) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.sweep()
	b, ok := mb.boxes[owner][mailbox]
	if !ok {
		return nil, false
	}
	delete(mb.boxes[owner], mailbox)
	return b.box, true
}

// sweep forgets the boxes past their time. mb.mu is held.
func (mb *mailboxes) sweep() {
	now := mb.now()
	for owner, mine := range mb.boxes {
		for id, b := range mine {
			if now.After(b.expires) {
				delete(mine, id)
			}
		}
		if len(mine) == 0 {
			delete(mb.boxes, owner)
		}
	}
}

// validMailbox reports whether id names a mailbox: a hash of a code, as 32
// hex digits.
func validMailbox(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 16
}

// Key is the id of the requester's key, "" while they have none: a client
// learns from it whether to seal, and whether it must pair.
func (m *Midgard) Key(c *gin.Context) {
	rm, err := m.rel().room(c.Request.Context(), c.GetString(ctxOwner))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	rm.mu.Lock()
	kid := rm.kid
	rm.mu.Unlock()
	c.JSON(http.StatusOK, types.KeyOutput{Kid: kid})
}

// LeavePairing leaves a pairing box for a new device of the requester's.
func (m *Midgard) LeavePairing(c *gin.Context) {
	var in types.PairInput
	if err := c.ShouldBindJSON(&in); err != nil || !validMailbox(in.Mailbox) {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "a pairing needs its mailbox, 32 hex digits, and its box"})
		return
	}
	box, err := base64.StdEncoding.DecodeString(in.Box)
	if err != nil || len(box) == 0 || len(box) > boxMax {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "a pairing box is at most 256 bytes, in base64"})
		return
	}
	if !m.rel().pairs.leave(c.GetString(ctxOwner), in.Mailbox, box) {
		c.JSON(http.StatusTooManyRequests, gin.H{"msg": "too many pairings wait: use one, or wait ten minutes"})
		return
	}
	c.Status(http.StatusNoContent)
}

// TakePairing hands a new device of the requester's the box its code names,
// once.
func (m *Midgard) TakePairing(c *gin.Context) {
	id := c.Param("mailbox")
	if !validMailbox(id) {
		c.JSON(http.StatusNotFound, gin.H{"msg": "no such pairing"})
		return
	}
	box, ok := m.rel().pairs.take(c.GetString(ctxOwner), id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"msg": "no such pairing: it was used, or ten minutes passed; show a new code"})
		return
	}
	c.JSON(http.StatusOK, types.PairOutput{Box: base64.StdEncoding.EncodeToString(box)})
}
