// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/types"
	"github.com/gorilla/websocket"
	"latere.ai/x/pkg/authkit/issuertest"
)

// person is someone signed in, by the token they send.
type person struct{ sub, email string }

func (p person) header() string {
	return "Bearer " + testIssuer.Mint(issuertest.Claims{Sub: p.sub, Email: p.email})
}

func (p person) request(t *testing.T, srv *httptest.Server, method, path, body string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	req.Header.Set("Authorization", p.header())
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

// daemon connects a daemon named id for p, and returns its connection.
func (p person) daemon(t *testing.T, srv *httptest.Server, id string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/midgard/api/v1/ws"
	c, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Authorization": {p.header()}})
	if err != nil {
		t.Fatalf("%s cannot subscribe: %v", p.sub, err)
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

// heard reports the text of the next clipboard change c hears within wait,
// or "" if none arrives.
func heard(c *websocket.Conn, wait time.Duration) string {
	c.SetReadDeadline(time.Now().Add(wait))
	defer c.SetReadDeadline(time.Time{})
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			return ""
		}
		wsm := &types.WebsocketMessage{}
		if wsm.Decode(msg) != nil || wsm.Action != types.ActionClipboardChanged {
			continue
		}
		var d types.ClipboardData
		json.Unmarshal(wsm.Data, &d)
		return d.Data
	}
}

// TestClipboardsAreApart is the hard barrier (specs/redesign.md §5) on the
// live path: two people, two daemons each. A copy reaches its owner's other
// devices and no one else's, the clipboard one reads is one's own, and so is
// the list of devices.
func TestClipboardsAreApart(t *testing.T) {
	resetBlocklist(t)
	t.Chdir(t.TempDir())
	t.Setenv("AUTH_ALLOWED_PRINCIPALS", "alice@example.com, bob@example.com")
	m := testMidgard(t)
	withStore(t, m)
	srv := httptest.NewServer(m.routers())
	t.Cleanup(srv.Close)

	alice := person{"sub-alice", "alice@example.com"}
	bob := person{"sub-bob", "bob@example.com"}
	aliceLaptop := alice.daemon(t, srv, "laptop")
	alicePhone := alice.daemon(t, srv, "phone")
	bobLaptop := bob.daemon(t, srv, "laptop") // the same name: names are per person
	bob.daemon(t, srv, "desktop")

	// alice copies on her laptop
	aliceLaptop.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
		Action: types.ActionClipboardPut, UserID: "laptop",
		Data: []byte(`{"type":"text/plain","data":"alice's secret","daemon_id":"laptop"}`),
	}).Encode())

	if got := heard(alicePhone, 3*time.Second); got != "alice's secret" {
		t.Fatalf("alice's phone heard %q, want her copy", got)
	}
	if got := heard(bobLaptop, 500*time.Millisecond); got != "" {
		t.Fatalf("bob's laptop heard alice's copy: %q", got)
	}

	// bob reads the clipboard: his own, empty, not alice's
	code, body := bob.request(t, srv, http.MethodGet, "/midgard/api/v1/clipboard", "")
	if code != http.StatusOK || strings.Contains(string(body), "alice's secret") {
		t.Fatalf("bob's clipboard: %d %s", code, body)
	}
	code, body = alice.request(t, srv, http.MethodGet, "/midgard/api/v1/clipboard", "")
	if code != http.StatusOK || !strings.Contains(string(body), "alice's secret") {
		t.Fatalf("alice's clipboard: %d %s, want her copy", code, body)
	}

	// bob copies through the REST API; alice hears nothing of it
	bob.request(t, srv, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text/plain","data":"bob's"}`)
	if got := heard(alicePhone, 500*time.Millisecond); got != "" {
		t.Fatalf("alice's phone heard bob's copy: %q", got)
	}

	// each sees only their own devices
	for p, want := range map[person][]string{alice: {"laptop", "phone"}, bob: {"laptop", "desktop"}} {
		_, body := p.request(t, srv, http.MethodGet, "/midgard/api/v1/devices", "")
		var out types.DevicesOutput
		json.Unmarshal(body, &out)
		var names []string
		for _, d := range out.Devices {
			names = append(names, d.Name)
		}
		if strings.Join(names, ",") != strings.Join(want, ",") {
			t.Errorf("%s's devices = %v, want %v", p.sub, names, want)
		}
	}
}
