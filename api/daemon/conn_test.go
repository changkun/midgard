// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/types/proto"
	"github.com/gorilla/websocket"
)

// fast are connection timings short enough for a test.
var fast = keepalive{
	ping:     50 * time.Millisecond,
	wait:     300 * time.Millisecond,
	write:    time.Second,
	retryMin: 10 * time.Millisecond,
	retryMax: 50 * time.Millisecond,
}

// fakeServer answers the registration handshake and then hands each
// connection, numbered from 1, to session.
func fakeServer(t *testing.T, session func(n int64, c *websocket.Conn)) (url string, conns *atomic.Int64) {
	t.Helper()
	conns = new(atomic.Int64)
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		if _, _, err := c.ReadMessage(); err != nil {
			return
		}
		c.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
			Action: types.ActionHandshakeReady, UserID: "test",
		}).Encode())
		session(conns.Add(1), c)
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http"), conns
}

func run(t *testing.T, url string) *Daemon {
	t.Helper()
	m := NewDaemon()
	m.ID, m.url, m.keepalive = "test", url, fast
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); m.stayConnected(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	return m
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting until %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestReconnectsAgainAndAgain is #33. The daemon reconnected once; the second
// reconnection blocked forever on a channel nothing read, and a failed write
// ended the writer for good. Either way it stopped syncing while it still
// looked alive.
func TestReconnectsAgainAndAgain(t *testing.T) {
	got := make(chan string, 1)
	url, conns := fakeServer(t, func(n int64, c *websocket.Conn) {
		if n <= 3 {
			return // drop the connection straight after the handshake
		}
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			wsm := &types.WebsocketMessage{}
			if wsm.Decode(msg) == nil && wsm.Action == types.ActionClipboardPut {
				got <- string(wsm.Data)
				return
			}
		}
	})
	m := run(t, url)

	eventually(t, "the daemon reconnected three times", func() bool { return conns.Load() >= 4 })
	m.writeCh <- &types.WebsocketMessage{Action: types.ActionClipboardPut, Data: []byte("after")}
	select {
	case d := <-got:
		if d != "after" {
			t.Fatalf("server got %q, want %q", d, "after")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a write after reconnecting never reached the server")
	}
}

// TestReconnectsFromASilentServer covers the connection that stays open but
// carries nothing, which is what a network change leaves behind: nothing was
// ever read from it, so the daemon waited on it forever.
func TestReconnectsFromASilentServer(t *testing.T) {
	url, conns := fakeServer(t, func(n int64, c *websocket.Conn) {
		if n == 1 {
			// Say nothing and answer no ping, without closing.
			time.Sleep(3 * time.Second)
			return
		}
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	})
	run(t, url)
	eventually(t, "the daemon gave up on the silent connection", func() bool { return conns.Load() >= 2 })
}

// TestListDaemonsTimeoutDoesNotStopSync: a timed-out mg daemon ls left its
// reply channel registered, unbuffered, and the read loop then blocked on it
// with the next message, for good.
func TestListDaemonsTimeoutDoesNotStopSync(t *testing.T) {
	answer := make(chan struct{})
	url, _ := fakeServer(t, func(n int64, c *websocket.Conn) {
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			wsm := &types.WebsocketMessage{}
			if wsm.Decode(msg) != nil || wsm.Action != types.ActionListDaemonsRequest {
				continue
			}
			select {
			case <-answer: // answer only once the first request has timed out
				c.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
					Action: types.ActionListDaemonsResponse, Data: []byte("id\tname\n"),
				}).Encode())
			default:
				// A stray reply the first reader is no longer waiting for.
				c.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
					Action: types.ActionNone,
				}).Encode())
			}
		}
	})
	m := run(t, url)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := m.ListDaemons(ctx, &proto.ListDaemonsInput{}); err == nil {
		t.Fatal("the first request should have timed out")
	}
	close(answer)

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := m.ListDaemons(ctx, &proto.ListDaemonsInput{})
	if err != nil {
		t.Fatalf("a request after a timed-out one failed: %v", err)
	}
	if out.Daemons != "id\tname\n" {
		t.Fatalf("got %q", out.Daemons)
	}
}
