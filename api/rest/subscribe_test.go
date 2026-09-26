// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/types"
	"github.com/gorilla/websocket"
)

// subscribe registers a daemon named id with the server at srv.
func subscribe(t *testing.T, srv *httptest.Server, id string) *websocket.Conn {
	t.Helper()
	h := http.Header{"Authorization": {bearer()}}
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/midgard/api/v1/ws"
	c, _, err := websocket.DefaultDialer.Dial(url, h)
	if err != nil {
		t.Fatalf("cannot subscribe: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	c.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
		Action: types.ActionHandshakeRegister, UserID: id,
	}).Encode())
	if _, _, err := c.ReadMessage(); err != nil {
		t.Fatalf("no handshake reply: %v", err)
	}
	return c
}

func (m *Midgard) subscribers() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.users.Len()
}

// TestServerDropsSilentDaemons: a daemon that vanished without closing its
// connection stayed subscribed forever, listed by mg daemon ls and written to
// by every broadcast.
func TestServerDropsSilentDaemons(t *testing.T) {
	resetBlocklist(t)
	m := NewMidgard()
	m.keepalive = keepalive{ping: 50 * time.Millisecond, wait: 300 * time.Millisecond, write: time.Second}
	srv := httptest.NewServer(m.routers())
	t.Cleanup(srv.Close)

	// The live daemon keeps reading, which answers the server's pings.
	live := subscribe(t, srv, "live")
	go func() {
		for {
			if _, _, err := live.ReadMessage(); err != nil {
				return
			}
		}
	}()
	// The silent one never reads again, so it answers nothing.
	subscribe(t, srv, "silent")

	deadline := time.Now().Add(5 * time.Second)
	for m.subscribers() != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("still %d subscribers; the silent daemon was never dropped", m.subscribers())
		}
		time.Sleep(20 * time.Millisecond)
	}
	// and the live one is still there well after the silent one's timeout
	time.Sleep(3 * m.keepalive.wait)
	if n := m.subscribers(); n != 1 {
		t.Fatalf("got %d subscribers, want the live daemon kept", n)
	}
}
