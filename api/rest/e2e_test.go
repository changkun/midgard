// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/e2e"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/wire"
	"github.com/gorilla/websocket"
)

// The tests of end-to-end encryption (specs/redesign.md §11): devices that
// seal, and a server that passes what it cannot read.

// sealing gives the device somewhere to keep a key, in memory, which makes
// it a device that seals: the first of its person's makes their key.
func (d *testDevice) sealing() *testDevice {
	var mu sync.Mutex
	d.e.SaveKey = func(k *e2e.Key, since uint64) error {
		mu.Lock()
		defer mu.Unlock()
		d.e.Key, d.e.Since = k, since
		return nil
	}
	return d
}

// startUnpaired starts a device that must pair, and waits until it knows.
func (d *testDevice) startUnpaired() *testDevice {
	d.t.Helper()
	d.start0()
	eventually(d.t, d.name+" is told to pair", func() bool { p, _ := d.e.NeedsPairing(); return p })
	return d
}

// start0 starts the device without waiting for it to be online.
func (d *testDevice) start0() {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel, d.done = cancel, make(chan struct{})
	go func() { defer close(d.done); d.e.Run(ctx) }()
}

// hears reports whether the device hears want within a few seconds,
// whatever it hears before: a copy made earlier may reach its clipboard
// first, on a slower machine.
func (d *testDevice) hears(want string) bool {
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-d.changed:
			if string(e.Data) == want {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// sealedRequest is a request from a client that opens sealed copies.
func (s *server) sealedRequest(p person, method, path, body string) (int, []byte) {
	s.t.Helper()
	req, _ := http.NewRequest(method, s.url()+path, strings.NewReader(body))
	req.Header.Set("Authorization", p.header())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(types.HeaderSealed, "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

// pair pairs to from from, as a person does with a code: from leaves the
// key in a mailbox, to takes it.
func pair(t *testing.T, s *server, p person, from, to *testDevice) {
	t.Helper()
	k, since := from.e.PersonKey()
	code, _ := e2e.NewCode()
	box, _ := code.Seal(k, since)
	in, _ := json.Marshal(types.PairInput{Mailbox: code.Mailbox(), Box: base64.StdEncoding.EncodeToString(box)})
	if status, b := s.request(p, http.MethodPost, "/midgard/api/v1/pair", string(in)); status != http.StatusNoContent {
		t.Fatalf("leaving the pairing box: %d %s", status, b)
	}
	given, _ := e2e.ParseCode(code.String())
	status, b := s.request(p, http.MethodGet, "/midgard/api/v1/pair/"+given.Mailbox(), "")
	var out types.PairOutput
	json.Unmarshal(b, &out)
	got, _ := base64.StdEncoding.DecodeString(out.Box)
	key, since, err := given.Open(got)
	if status != http.StatusOK || err != nil {
		t.Fatalf("taking the pairing box: %d %s, %v", status, b, err)
	}
	if err := to.e.SetKey(key, since); err != nil {
		t.Fatal(err)
	}
}

func TestSealed(t *testing.T) {
	s := twoPeople(t)

	// the first device to seal makes alice's key, and the server learns
	// its id, never the key
	laptop := alice.device(t, s, "laptop").sealing().start()
	eventually(t, "the laptop seals", laptop.e.Sealing)
	key, _ := laptop.e.PersonKey()
	if _, b := s.request(alice, http.MethodGet, "/midgard/api/v1/key", ""); !strings.Contains(string(b), key.ID()) {
		t.Fatalf("GET /key = %s, want %s", b, key.ID())
	}

	// what the server holds of a copy is sealed
	laptop.copy("the secret words")
	rm, _ := s.m.rel().room(t.Context(), alice.sub)
	eventually(t, "the server holds the copy", func() bool { _, ok := rm.newest(); return ok })
	held, _ := rm.newest()
	if held.Kid != key.ID() || bytes.Contains(held.Payload, []byte("secret")) || held.Size() != len("the secret words") {
		t.Fatalf("the server holds %q, kid %q, size %d: want it sealed with %s", held.Payload, held.Kid, held.Size(), key.ID())
	}

	// a second device lacks the key: it is told to pair, and syncs nothing
	phone := alice.device(t, s, "phone").sealing().startUnpaired()
	if _, kid := phone.e.NeedsPairing(); kid != key.ID() {
		t.Fatalf("the phone was told the key %q, want %s", kid, key.ID())
	}
	if phone.texts() != "" {
		t.Fatalf("an unpaired phone has %q", phone.texts())
	}

	// paired, it catches up, and the two sync both ways
	pair(t, s, alice, laptop, phone)
	eventually(t, "the phone is online and seals", func() bool { return phone.e.Online() && phone.e.Sealing() })
	eventually(t, "the phone has the laptop's copy", func() bool { return phone.texts() == "the secret words" })
	phone.copy("from the phone")
	if !laptop.hears("from the phone") {
		t.Fatal("the laptop did not hear the phone's copy")
	}

	// a client that does not open sealed copies is refused them, rather
	// than handed ciphertext
	for _, path := range []string{"/midgard/api/v1/clipboard", "/midgard/api/v1/history"} {
		if code, _ := s.request(alice, http.MethodGet, path, ""); code != http.StatusConflict {
			t.Errorf("GET %s without saying it opens sealed copies: %d, want 409", path, code)
		}
	}
	// one that does gets the sealed bytes, which only the key opens
	code, b := s.sealedRequest(alice, http.MethodGet, "/midgard/api/v1/clipboard", "")
	var clip types.ClipboardData
	json.Unmarshal(b, &clip)
	sealed, _ := base64.StdEncoding.DecodeString(clip.Data)
	plain, err := key.Open(e2e.Copy, []wire.Format{{MIME: "text", Size: len(sealed) - e2e.Overhead}}, sealed)
	if code != http.StatusOK || clip.Kid != key.ID() || clip.Device != "phone" || err != nil || string(plain) != "from the phone" {
		t.Fatalf("GET /clipboard = %d %s; opened %q, %v", code, b, plain, err)
	}
	code, b = s.sealedRequest(alice, http.MethodGet, "/midgard/api/v1/history", "")
	var h types.HistoryOutput
	json.Unmarshal(b, &h)
	if code != http.StatusOK || len(h.History) != 2 || h.History[1].Kid != key.ID() {
		t.Fatalf("GET /history = %d %s", code, b)
	}
	preview, _ := base64.StdEncoding.DecodeString(h.History[1].Preview)
	if got, err := key.Open(e2e.Preview, []wire.Format{{MIME: "text", Size: h.History[1].Size}}, preview); err != nil || string(got) != "the secret words" {
		t.Fatalf("the history's preview opened to %q, %v", got, err)
	}

	// a copy sent in the clear is refused; one sealed with the key passes
	if code, b := s.request(alice, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"in the clear"}`); code != http.StatusConflict {
		t.Errorf("a copy in the clear: %d %s, want 409", code, b)
	}
	other, _ := e2e.NewKey()
	for k, want := range map[*e2e.Key]int{other: http.StatusConflict, key: http.StatusOK} {
		sealed, _ := k.Seal(e2e.Copy, []wire.Format{{MIME: "text", Size: 8}}, []byte("from mg!"))
		body, _ := json.Marshal(types.PutToUniversalClipboardInput{ClipboardData: types.ClipboardData{
			Type: types.MIMEPlainText, Data: base64.StdEncoding.EncodeToString(sealed), Kid: k.ID(),
		}})
		if code, b := s.request(alice, http.MethodPost, "/midgard/api/v1/clipboard", string(body)); code != want {
			t.Errorf("a copy sealed with %s: %d %s, want %d", k.ID(), code, b, want)
		}
	}
	if !laptop.hears("from mg!") {
		t.Fatal("the laptop did not hear the sealed copy from mg")
	}

	// bob cannot take alice's pairing box
	code2, _ := e2e.NewCode()
	box, _ := code2.Seal(key, 0)
	in, _ := json.Marshal(types.PairInput{Mailbox: code2.Mailbox(), Box: base64.StdEncoding.EncodeToString(box)})
	s.request(alice, http.MethodPost, "/midgard/api/v1/pair", string(in))
	if code, _ := s.request(bob, http.MethodGet, "/midgard/api/v1/pair/"+code2.Mailbox(), ""); code != http.StatusNotFound {
		t.Errorf("bob took alice's pairing box: %d", code)
	}
	// the device that left it can ask whether it waits, without taking it,
	// and no one else can
	for i := 0; i < 2; i++ {
		if code, _ := s.request(alice, http.MethodHead, "/midgard/api/v1/pair/"+code2.Mailbox(), ""); code != http.StatusNoContent {
			t.Errorf("alice's box waits, asked: %d, want 204", code)
		}
	}
	if code, _ := s.request(bob, http.MethodHead, "/midgard/api/v1/pair/"+code2.Mailbox(), ""); code != http.StatusGone {
		t.Errorf("bob learned alice's box waits: %d, want 410, as for any box not his", code)
	}
	if code, _ := s.request(alice, http.MethodGet, "/midgard/api/v1/pair/"+code2.Mailbox(), ""); code != http.StatusOK {
		t.Errorf("alice could not take her own box: %d", code)
	}
	if code, _ := s.request(alice, http.MethodGet, "/midgard/api/v1/pair/"+code2.Mailbox(), ""); code != http.StatusNotFound {
		t.Errorf("a box taken twice: %d, want 404", code)
	}
	if code, _ := s.request(alice, http.MethodHead, "/midgard/api/v1/pair/"+code2.Mailbox(), ""); code != http.StatusGone {
		t.Errorf("a box taken, asked: %d, want 410, for its device to offer a new code", code)
	}
}

// TestOlderDevicesOnceSealed: a device that cannot seal is let in while its
// person has no key, and sent away once they have one, before it takes a
// sealed copy for text.
func TestOlderDevicesOnceSealed(t *testing.T) {
	s := twoPeople(t)
	hello := func() (wire.Frame, string) {
		url := "ws" + strings.TrimPrefix(s.url(), "http") + "/midgard/api/v1/ws"
		c, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Authorization": {alice.header()}})
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		b, _ := (&wire.Frame{Envelope: wire.Envelope{Type: wire.Hello, V: 1, Device: "old", Name: "old", Clock: time.Now().UnixMilli()}}).Marshal()
		c.WriteMessage(websocket.BinaryMessage, b)
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, b, err = c.ReadMessage()
		if err != nil {
			var ce *websocket.CloseError
			if errors.As(err, &ce) {
				return wire.Frame{}, ce.Text
			}
			return wire.Frame{}, err.Error()
		}
		f, _ := wire.Unmarshal(b)
		return f, ""
	}
	if f, reason := hello(); f.Type != wire.Welcome {
		t.Fatalf("an older device, before any key: %+v %q, want it welcomed", f, reason)
	}
	laptop := alice.device(t, s, "laptop").sealing().start()
	eventually(t, "the laptop seals", laptop.e.Sealing)
	if f, reason := hello(); f.Type == wire.Welcome || !strings.Contains(reason, "update midgard") {
		t.Fatalf("an older device, once alice has a key: %+v %q, want it told to update", f, reason)
	}
}

// TestShortcutsBridge: a device switched on as a bridge hands the Shortcuts
// its newest copy in the clear, and seals in what they send, in their name;
// with no bridge online, the Shortcuts get nothing.
func TestShortcutsBridge(t *testing.T) {
	s := twoPeople(t)
	if code, _ := s.request(alice, http.MethodGet, "/midgard/api/v1/plain/clipboard", ""); code != http.StatusServiceUnavailable {
		t.Fatalf("GET /plain/clipboard with no bridge: %d, want 503", code)
	}

	mac := alice.device(t, s, "mac").sealing()
	mac.e.Bridge = true
	mac.start()
	eventually(t, "the mac seals", mac.e.Sealing)
	laptop := alice.device(t, s, "laptop").sealing().startUnpaired()
	pair(t, s, alice, mac, laptop)
	eventually(t, "the laptop is online", laptop.e.Online)

	// the bridge's newest copy, in the clear, for Get from Midgard
	mac.copy("for the iphone")
	var clip types.ClipboardData
	eventually(t, "the Shortcuts see the newest copy", func() bool {
		code, b := s.request(alice, http.MethodGet, "/midgard/api/v1/plain/clipboard", "")
		json.Unmarshal(b, &clip)
		return code == http.StatusOK && clip.Data == "for the iphone"
	})
	if clip.Kid != "" {
		t.Errorf("the Shortcuts got a sealed copy: %+v", clip)
	}

	// what Send to Midgard sends, sealed in by the bridge, in its name
	if code, b := s.request(alice, http.MethodPost, "/midgard/api/v1/plain/clipboard", `{"type":"text","data":"from the iphone"}`); code != http.StatusOK {
		t.Fatalf("POST /plain/clipboard: %d %s", code, b)
	}
	if !laptop.hears("from the iphone") {
		t.Fatal("the laptop did not hear what the Shortcut sent")
	}
	// it came through sealed, as the relay takes nothing else now, and in
	// the Shortcut's name
	if n, ok, _ := laptop.e.History.Newest(t.Context(), false); !ok || n.Origin != "Shortcuts" {
		t.Errorf("the laptop has it from %q, want Shortcuts", n.Origin)
	}

	// the bridge gone, the copy in the clear goes too
	mac.stop()
	eventually(t, "the Shortcuts see nothing", func() bool {
		code, _ := s.request(alice, http.MethodGet, "/midgard/api/v1/plain/clipboard", "")
		return code == http.StatusServiceUnavailable
	})
	if code, _ := s.request(alice, http.MethodPost, "/midgard/api/v1/plain/clipboard", `{"type":"text","data":"x"}`); code != http.StatusServiceUnavailable {
		t.Errorf("POST /plain/clipboard with no bridge: %d, want 503", code)
	}
}
