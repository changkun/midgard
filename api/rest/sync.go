// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"changkun.de/x/midgard/internal/wire"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// keepalive is how the server notices devices that went away without closing
// their connection.
type keepalive struct {
	// ping is how often the server pings each device.
	ping time.Duration
	// wait is how long a device may stay silent before the server drops
	// it. Without it a dead device stayed listed forever, and every
	// broadcast kept writing to it.
	wait time.Duration
	// write bounds a single write, so one stuck device cannot hold up the
	// others.
	write time.Duration
}

var defaultKeepalive = keepalive{ping: 30 * time.Second, wait: 90 * time.Second, write: 10 * time.Second}

// Sync is the websocket a device stays connected on (specs/redesign.md §8).
// It joins its owner's room, named by its sign-in and never by anything it
// says, and hears only what happens to its owner's history.
func (m *Midgard) Sync(c *gin.Context) {
	owner := c.GetString(ctxOwner)
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("cannot upgrade the connection", "err", err)
		return
	}
	defer conn.Close()
	conn.SetReadLimit(wire.MaxPayload + 1<<16)

	ka := m.keepalive
	conn.SetReadDeadline(time.Now().Add(ka.wait))
	_, b, err := conn.ReadMessage()
	if err != nil {
		return
	}
	hello, err := wire.Unmarshal(b)
	refuse := func(reason string) {
		conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseProtocolError, reason), time.Now().Add(ka.write))
	}
	if err != nil || hello.Type != wire.Hello || hello.Device == "" {
		reason := "the first frame must be a hello that names the device"
		if hello.Type == wire.Hello && (hello.V < wire.MinVersion || hello.V > wire.Version) {
			reason = "this server speaks another version; update midgard"
		}
		refuse(reason)
		return
	}
	if hello.V < wire.MinVersion || hello.V > wire.Version {
		refuse("this server speaks another version; update midgard")
		return
	}

	ctx := c.Request.Context()
	r := m.rel()
	rm, err := r.room(ctx, owner)
	if err != nil {
		slog.Error("cannot open the room", "err", err)
		return
	}
	switch kid, err := r.admit(ctx, rm, hello); {
	case errors.Is(err, errUpdate):
		refuse(err.Error())
		return
	case errors.Is(err, errPair):
		// the device learns the key it lacks, and pairs
		conn.SetWriteDeadline(time.Now().Add(ka.write))
		conn.WriteMessage(websocket.BinaryMessage, encode(wire.Frame{Envelope: wire.Envelope{
			Type: wire.Welcome, V: wire.Version, Kid: kid, Err: err.Error(),
		}}))
		conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "pair this device"), time.Now().Add(ka.write))
		return
	case err != nil:
		slog.Error("cannot let a device in", "err", err)
		return
	}
	l := &link{
		id: hello.Device, name: hello.Name, v: hello.V, bridge: hello.Bridge,
		out: make(chan []byte, 256), gone: make(chan struct{}),
		offset: time.Since(time.UnixMilli(hello.Clock)),
	}
	if l.name == "" {
		l.name = l.id
	}
	if err := r.join(ctx, rm, l, hello); err != nil {
		slog.Error("cannot let a device in", "err", err)
		return
	}
	defer r.leave(ctx, rm, l)
	slog.Info("a device connected", "device", l.name)
	defer slog.Info("a device disconnected", "device", l.name)
	go r.fill(ctx, rm)

	// This goroutine writes; the one below reads. Anything the device sends
	// proves it is still there.
	alive := func() { conn.SetReadDeadline(time.Now().Add(ka.wait)) }
	alive()
	conn.SetPongHandler(func(string) error { alive(); return nil })
	conn.SetPingHandler(func(data string) error {
		alive()
		err := conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(ka.write))
		if errors.Is(err, websocket.ErrCloseSent) {
			return nil
		}
		return err
	})
	go func() {
		defer l.close()
		for {
			_, b, err := conn.ReadMessage()
			if err != nil {
				return
			}
			alive()
			f, err := wire.Unmarshal(b)
			if err != nil {
				slog.Warn("a device sent a frame that is not one", "device", l.name, "err", err)
				return
			}
			switch f.Type {
			case wire.Copy, wire.Delete, wire.Clear:
				if _, err := r.number(ctx, rm, l, l.name, f); err != nil {
					slog.Error("cannot number what a device sent", "device", l.name, "err", err)
				}
			case wire.Ack:
				r.ack(ctx, rm, l, f.Acked, f.Gaps)
			case wire.Have, wire.Done:
				r.answer(rm, l, f)
			case wire.Mirror:
				r.setMirror(rm, l, f)
			}
		}
	}()

	ping := time.NewTicker(ka.ping)
	defer ping.Stop()
	for {
		select {
		case <-l.gone:
			return
		case <-ping.C:
			if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(ka.write)) != nil {
				return
			}
		case b := <-l.out:
			conn.SetWriteDeadline(time.Now().Add(ka.write))
			if err := conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
				return
			}
		}
	}
}

// Devices lists the requester's devices, online or not.
func (m *Midgard) Devices(c *gin.Context) {
	out, err := m.rel().devices(c.Request.Context(), c.GetString(ctxOwner))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}
