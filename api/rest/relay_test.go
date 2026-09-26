// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/device"
	"changkun.de/x/midgard/internal/history"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/wire"
	"github.com/gorilla/websocket"
)

// The relay's tests run the devices' real sync (internal/device) against
// the server, a person's several devices at once.

var fastSync = device.Keepalive{
	Ping: 50 * time.Millisecond, Wait: 2 * time.Second, Write: time.Second,
	RetryMin: 10 * time.Millisecond, RetryMax: 50 * time.Millisecond, Settle: 20 * time.Millisecond,
}

// server is a midgard server for a test, at a URL a test can move, to
// restart it.
type server struct {
	t   *testing.T
	m   *Midgard
	mu  sync.Mutex
	srv *httptest.Server
}

func newServer(t *testing.T, m *Midgard) *server {
	s := &server{t: t, m: m, srv: httptest.NewServer(m.routers())}
	t.Cleanup(func() { s.mu.Lock(); s.srv.Close(); s.mu.Unlock() })
	return s
}

func (s *server) url() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.srv.URL
}

// restart stops the server, dropping every connection and all it holds in
// memory, and starts another on the same store.
func (s *server) restart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m.rel().closeAll()
	s.srv.Close()
	m := NewMidgard()
	m.store, m.keepalive = s.m.store, s.m.keepalive
	s.m = m
	s.srv = httptest.NewServer(m.routers())
}

func (s *server) request(p person, method, path, body string) (int, []byte) {
	s.t.Helper()
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	return p.request(s.t, srv, method, path, body)
}

// testDevice is one of a person's devices.
type testDevice struct {
	t       *testing.T
	name    string
	e       *device.Engine
	changed chan history.Entry
	cancel  context.CancelFunc
	done    chan struct{}
}

func (p person) device(t *testing.T, s *server, name string) *testDevice {
	t.Helper()
	h, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	d := &testDevice{t: t, name: name, changed: make(chan history.Entry, 64)}
	d.e = &device.Engine{
		ID: p.sub + "-" + name, Name: name, History: h, Keepalive: fastSync,
		Changed: func(e history.Entry) { d.changed <- e },
		Dial: func(ctx context.Context) (*websocket.Conn, error) {
			url := "ws" + strings.TrimPrefix(s.url(), "http") + "/midgard/api/v1/ws"
			c, _, err := websocket.DefaultDialer.DialContext(ctx, url, http.Header{"Authorization": {p.header()}})
			return c, err
		},
	}
	t.Cleanup(d.stop)
	return d
}

func (d *testDevice) start() *testDevice {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel, d.done = cancel, make(chan struct{})
	go func() { defer close(d.done); d.e.Run(ctx) }()
	eventually(d.t, d.name+" is online", d.e.Online)
	return d
}

func (d *testDevice) stop() {
	if d.cancel != nil {
		d.cancel()
		<-d.done
		d.cancel = nil
	}
}

func (d *testDevice) copy(text string) {
	d.t.Helper()
	if err := d.e.Copy(context.Background(), "text", []byte(text)); err != nil {
		d.t.Fatal(err)
	}
}

// texts is the device's history as its texts, newest first; a copy still
// waiting to be numbered in parentheses.
func (d *testDevice) texts() string {
	d.t.Helper()
	ctx := context.Background()
	list, err := d.e.History.List(ctx, 1000)
	if err != nil {
		d.t.Fatal(err)
	}
	var out []string
	for _, e := range list {
		if e.Waiting() {
			out = append(out, "(waiting)")
			continue
		}
		got, err := d.e.History.Get(ctx, e.Seq)
		if errors.Is(err, history.ErrNotFound) {
			continue // removed between the listing and now
		}
		if err != nil {
			d.t.Fatal(err)
		}
		out = append(out, string(got.Data))
	}
	return strings.Join(out, " ")
}

// heard is the text of the next copy that arrives for the clipboard within
// wait, or "".
func (d *testDevice) heard(wait time.Duration) string {
	select {
	case e := <-d.changed:
		return string(e.Data)
	case <-time.After(wait):
		return ""
	}
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

func (s *server) queue(p person) types.QueueOutput {
	s.t.Helper()
	code, b := s.request(p, http.MethodGet, "/midgard/api/v1/queue", "")
	var out types.QueueOutput
	if json.Unmarshal(b, &out); code != http.StatusOK {
		s.t.Fatalf("queue: %d %s", code, b)
	}
	return out
}

var alice = person{"sub-alice", "alice@example.com"}
var bob = person{"sub-bob", "bob@example.com"}

// twoPeople is a server alice and bob may use.
func twoPeople(t *testing.T) *server {
	t.Helper()
	resetBlocklist(t)
	t.Setenv("AUTH_ALLOWED_PRINCIPALS", "alice@example.com, bob@example.com")
	return newServer(t, testMidgard(t))
}

// TestRelay: a copy reaches its person's other devices, in the same place of
// the same history on each, and the server forgets it once all have it.
func TestRelay(t *testing.T) {
	s := twoPeople(t)
	laptop := alice.device(t, s, "laptop").start()
	phone := alice.device(t, s, "phone").start()

	laptop.copy("hello")
	if got := phone.heard(3 * time.Second); got != "hello" {
		t.Fatalf("the phone heard %q", got)
	}
	// the laptop made it: it is on its clipboard already
	if got := laptop.heard(200 * time.Millisecond); got != "" {
		t.Fatalf("the laptop was told to put %q on its clipboard", got)
	}
	eventually(t, "both have it, numbered", func() bool { return laptop.texts() == "hello" && phone.texts() == "hello" })
	eventually(t, "the server forgot it", func() bool { return len(s.queue(alice).Queue) == 0 })

	// a copy from the web page reaches every device
	s.request(alice, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"from the web"}`)
	for _, d := range []*testDevice{laptop, phone} {
		if got := d.heard(3 * time.Second); got != "from the web" {
			t.Fatalf("%s heard %q", d.name, got)
		}
	}
	eventually(t, "the same history on both", func() bool {
		return laptop.texts() == "from the web hello" && phone.texts() == "from the web hello"
	})
}

// TestClipboardsAreApart is the hard barrier (specs/redesign.md §5): two
// people, two devices each. A copy reaches its owner's other devices and no
// one else's, what one reads is one's own, and so is the list of devices.
func TestClipboardsAreApart(t *testing.T) {
	s := twoPeople(t)
	aliceLaptop := alice.device(t, s, "laptop").start()
	alicePhone := alice.device(t, s, "phone").start()
	bobLaptop := bob.device(t, s, "laptop").start() // the same name: names are per person
	bob.device(t, s, "desktop").start()

	aliceLaptop.copy("alice's secret")
	if got := alicePhone.heard(3 * time.Second); got != "alice's secret" {
		t.Fatalf("alice's phone heard %q", got)
	}
	if got := bobLaptop.heard(300 * time.Millisecond); got != "" {
		t.Fatalf("bob's laptop heard alice's copy: %q", got)
	}
	eventually(t, "alice's laptop has it numbered", func() bool { return aliceLaptop.texts() == "alice's secret" })

	// bob reads his clipboard and history, from his devices: not alice's
	for _, path := range []string{"/midgard/api/v1/clipboard", "/midgard/api/v1/history", "/midgard/api/v1/queue", "/midgard/api/v1/history/1"} {
		_, body := s.request(bob, http.MethodGet, path, "")
		if strings.Contains(string(body), "alice's secret") || strings.Contains(string(body), "phone") {
			t.Errorf("bob's %s shows alice's: %s", path, body)
		}
	}
	code, body := s.request(alice, http.MethodGet, "/midgard/api/v1/clipboard", "")
	if code != http.StatusOK || !strings.Contains(string(body), "alice's secret") {
		t.Fatalf("alice's clipboard: %d %s", code, body)
	}

	// bob copies through the API, and deletes and clears; alice hears none of it
	s.request(bob, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"bob's"}`)
	s.request(bob, http.MethodDelete, "/midgard/api/v1/history/1", "")
	s.request(bob, http.MethodDelete, "/midgard/api/v1/history", "")
	if got := alicePhone.heard(300 * time.Millisecond); got != "" {
		t.Fatalf("alice's phone heard bob's copy: %q", got)
	}
	time.Sleep(100 * time.Millisecond)
	if got := alicePhone.texts(); got != "alice's secret" {
		t.Fatalf("bob's delete and clear reached alice: %q", got)
	}
	// bob cannot take back or forget what is alice's
	if code, _ := s.request(bob, http.MethodDelete, "/midgard/api/v1/devices/"+aliceLaptop.e.ID, ""); code != http.StatusNotFound {
		t.Errorf("bob forgetting alice's laptop: %d, want 404", code)
	}

	for p, want := range map[person]string{alice: "laptop,phone", bob: "desktop,laptop"} {
		_, body := s.request(p, http.MethodGet, "/midgard/api/v1/devices", "")
		var out types.DevicesOutput
		json.Unmarshal(body, &out)
		var names []string
		for _, d := range out.Devices {
			names = append(names, d.Name)
		}
		sortStrings(names)
		if strings.Join(names, ",") != want {
			t.Errorf("%s's devices = %v, want %v", p.sub, names, want)
		}
	}
}

func sortStrings(s []string) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// TestCatchUpFromTheQueue: a copy made while a device is off waits in the
// server's memory until it comes back (§6).
func TestCatchUpFromTheQueue(t *testing.T) {
	s := twoPeople(t)
	laptop := alice.device(t, s, "laptop").start()
	desktop := alice.device(t, s, "desktop").start()
	desktop.stop()

	laptop.copy("while you were away")
	eventually(t, "the copy waits for the desktop alone", func() bool {
		q := s.queue(alice)
		return len(q.Queue) == 1 && len(q.Queue[0].Waiting) == 1 && q.Queue[0].Waiting[0] == desktop.e.ID
	})

	desktop.start()
	if got := desktop.heard(3 * time.Second); got != "while you were away" {
		t.Fatalf("the desktop heard %q", got)
	}
	eventually(t, "the server forgot it", func() bool { return len(s.queue(alice).Queue) == 0 })
}

// TestOfflineCopyKeepsItsPlace: a copy made offline reaches the server after
// a newer one from another device. It takes its place at when it was made,
// and the newer stays the clipboard, on both (§6).
func TestOfflineCopyKeepsItsPlace(t *testing.T) {
	s := twoPeople(t)
	laptop := alice.device(t, s, "laptop").start()
	desktop := alice.device(t, s, "desktop").start()
	laptop.stop()

	laptop.copy("made offline")
	time.Sleep(50 * time.Millisecond)
	desktop.copy("made later")
	eventually(t, "the desktop's is numbered", func() bool { return desktop.texts() == "made later" })

	laptop.start()
	// the laptop catches up on the newer copy, which belongs on its clipboard
	if got := laptop.heard(3 * time.Second); got != "made later" {
		t.Fatalf("the laptop heard %q, want the newer copy", got)
	}
	want := "made later made offline"
	eventually(t, "both have both, in order", func() bool { return laptop.texts() == want && desktop.texts() == want })
	// the desktop learns of the older copy, and leaves its clipboard be
	if got := desktop.heard(300 * time.Millisecond); got != "" {
		t.Fatalf("the desktop put %q on its clipboard", got)
	}
}

// TestCatchUpFromAPeer: after a restart the server holds nothing; a device
// catching up gets what it lacks from another that has it.
func TestCatchUpFromAPeer(t *testing.T) {
	s := twoPeople(t)
	laptop := alice.device(t, s, "laptop").start()
	desktop := alice.device(t, s, "desktop").start()
	desktop.stop()
	laptop.copy("one")
	laptop.copy("two")
	eventually(t, "they are numbered", func() bool { return laptop.texts() == "two one" })

	s.restart()
	eventually(t, "the laptop is back, on the new server", func() bool {
		_, b := s.request(alice, http.MethodGet, "/midgard/api/v1/devices", "")
		return strings.Contains(string(b), `"online":true`)
	})
	if q := s.queue(alice); len(q.Queue) != 0 || q.Head != 2 {
		t.Fatalf("after a restart the queue is %+v, want empty at head 2", q)
	}
	desktop.start()
	eventually(t, "the desktop caught up from the laptop", func() bool { return desktop.texts() == "two one" })
}

// TestAGapNobodyHas: a device lacking what no one online can give it still
// acknowledges what it has, so the server forgets the rest.
func TestAGapNobodyHas(t *testing.T) {
	s := twoPeople(t)
	laptop := alice.device(t, s, "laptop").start()
	laptop.stop()
	// numbered while the laptop was off, then lost with a restart
	s.request(alice, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"lost"}`)
	s.restart()

	laptop.start()
	s.request(alice, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"after"}`)
	if got := laptop.heard(3 * time.Second); got != "after" {
		t.Fatalf("the laptop heard %q", got)
	}
	eventually(t, "the server forgot what the laptop has", func() bool { return len(s.queue(alice).Queue) == 0 })
	gaps, _ := laptop.e.History.Gaps(context.Background())
	if len(gaps) != 1 || gaps[0] != (wire.Span{From: 1, To: 1}) {
		t.Fatalf("gaps %v, want the lost copy", gaps)
	}
}

// TestTakeBack: a copy no device has yet can be taken back from the queue;
// the devices learn it is gone rather than that it is missing.
func TestTakeBack(t *testing.T) {
	s := twoPeople(t)
	laptop := alice.device(t, s, "laptop").start()
	laptop.stop()

	_, b := s.request(alice, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"oops"}`)
	var put types.PutToUniversalClipboardOutput
	json.Unmarshal(b, &put)
	path := "/midgard/api/v1/queue/" + itoa(put.Seq)
	if code, b := s.request(alice, http.MethodDelete, path, ""); code != http.StatusNoContent {
		t.Fatalf("taking it back: %d %s", code, b)
	}
	if code, _ := s.request(bob, http.MethodDelete, path, ""); code != http.StatusNotFound {
		t.Errorf("bob taking back alice's: %d", code)
	}

	laptop.start()
	s.request(alice, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"meant"}`)
	if got := laptop.heard(3 * time.Second); got != "meant" {
		t.Fatalf("the laptop heard %q", got)
	}
	eventually(t, "only the meant copy", func() bool { return laptop.texts() == "meant" })
	if gaps, _ := laptop.e.History.Gaps(context.Background()); len(gaps) != 0 {
		t.Fatalf("the void left a gap: %v", gaps)
	}
	// once a device has it, it is too late
	eventually(t, "the laptop has it", func() bool { return len(s.queue(alice).Queue) == 0 })
	laptop.stop()
	_, b = s.request(alice, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"x"}`)
	json.Unmarshal(b, &put)
	laptop.start()
	eventually(t, "it arrived", func() bool { return strings.HasPrefix(laptop.texts(), "x") })
	if code, _ := s.request(alice, http.MethodDelete, "/midgard/api/v1/queue/"+itoa(put.Seq), ""); code != http.StatusNotFound && code != http.StatusConflict {
		t.Fatalf("taking back what arrived: %d", code)
	}
	// nor while a device is online, which was sent it at once
	_, b = s.request(alice, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"y"}`)
	json.Unmarshal(b, &put)
	if code, _ := s.request(alice, http.MethodDelete, "/midgard/api/v1/queue/"+itoa(put.Seq), ""); code != http.StatusConflict {
		t.Fatalf("taking back with a device online: %d, want 409", code)
	}
}

func itoa(n uint64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// TestRemove: deleting a copy, or clearing, reaches every device, and
// changes nobody's clipboard.
func TestRemove(t *testing.T) {
	s := twoPeople(t)
	laptop := alice.device(t, s, "laptop").start()
	phone := alice.device(t, s, "phone").start()
	for _, text := range []string{"one", "two", "three"} {
		laptop.copy(text)
		phone.heard(3 * time.Second)
	}
	eventually(t, "both have all three", func() bool { return phone.texts() == "three two one" })

	if code, b := s.request(alice, http.MethodDelete, "/midgard/api/v1/history/3", ""); code != http.StatusNoContent {
		t.Fatalf("deleting the newest: %d %s", code, b)
	}
	eventually(t, "it is gone on both", func() bool { return laptop.texts() == "two one" && phone.texts() == "two one" })
	if got := phone.heard(200 * time.Millisecond); got != "" {
		t.Fatalf("deleting put %q on the phone's clipboard", got)
	}

	if err := phone.e.Delete(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a delete from the phone reaches the laptop", func() bool { return laptop.texts() == "two" })

	s.request(alice, http.MethodDelete, "/midgard/api/v1/history", "")
	eventually(t, "cleared on both", func() bool { return laptop.texts() == "" && phone.texts() == "" })
	if code, _ := s.request(alice, http.MethodDelete, "/midgard/api/v1/history/99", ""); code != http.StatusNotFound {
		t.Errorf("deleting a number never given out: %d", code)
	}
}

// TestReadsGoToADevice: the clipboard and the history are on the devices;
// the server asks one, and says so when none is online.
func TestReadsGoToADevice(t *testing.T) {
	s := twoPeople(t)
	for _, path := range []string{"/midgard/api/v1/clipboard", "/midgard/api/v1/history", "/midgard/api/v1/history/1"} {
		if code, _ := s.request(alice, http.MethodGet, path, ""); code != http.StatusServiceUnavailable {
			t.Errorf("GET %s with no device: %d, want 503", path, code)
		}
	}
	// what the server holds it can answer with alone
	s.request(alice, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"held"}`)
	if code, b := s.request(alice, http.MethodGet, "/midgard/api/v1/clipboard", ""); code != http.StatusOK || !strings.Contains(string(b), `"held"`) {
		t.Fatalf("the clipboard from what is held: %d %s", code, b)
	}

	laptop := alice.device(t, s, "laptop").start()
	if got := laptop.heard(3 * time.Second); got != "held" {
		t.Fatalf("the laptop heard %q", got)
	}
	laptop.copy("second")
	png := []byte("\x89PNG not really")
	laptop.e.Copy(context.Background(), "image/png", png)
	eventually(t, "the server forgot them", func() bool {
		return len(s.queue(alice).Queue) == 0 && !strings.Contains(laptop.texts(), "waiting")
	})

	code, b := s.request(alice, http.MethodGet, "/midgard/api/v1/history", "")
	var h types.HistoryOutput
	json.Unmarshal(b, &h)
	if code != http.StatusOK || len(h.History) != 3 || h.History[0].Type != types.MIMEImagePNG || h.History[0].Device != "laptop" || h.History[2].Device == "laptop" {
		t.Fatalf("history from the laptop: %d %s", code, b)
	}
	code, b = s.request(alice, http.MethodGet, "/midgard/api/v1/history/"+itoa(uint64(h.History[1].ID)), "")
	if code != http.StatusOK || !strings.Contains(string(b), `"second"`) {
		t.Fatalf("one copy from the laptop: %d %s", code, b)
	}
	code, b = s.request(alice, http.MethodGet, "/midgard/api/v1/clipboard", "")
	var clip types.ClipboardData
	json.Unmarshal(b, &clip)
	if code != http.StatusOK || clip.Type != types.MIMEImagePNG {
		t.Fatalf("the clipboard from the laptop: %d %s", code, b)
	}
	if code, _ := s.request(alice, http.MethodGet, "/midgard/api/v1/history/42", ""); code != http.StatusNotFound {
		t.Errorf("a copy no one has: %d, want 404", code)
	}
}

// TestForgetDevice: a device forgotten no longer holds copies in the queue.
func TestForgetDevice(t *testing.T) {
	s := twoPeople(t)
	laptop := alice.device(t, s, "laptop").start()
	old := alice.device(t, s, "old").start()
	old.stop()

	laptop.copy("x")
	eventually(t, "it waits for the old device", func() bool {
		q := s.queue(alice)
		return len(q.Queue) == 1 && len(q.Queue[0].Waiting) == 1
	})
	if code, b := s.request(alice, http.MethodDelete, "/midgard/api/v1/devices/"+old.e.ID, ""); code != http.StatusNoContent {
		t.Fatalf("forgetting: %d %s", code, b)
	}
	if q := s.queue(alice); len(q.Queue) != 0 {
		t.Fatalf("the queue still holds %+v", q.Queue)
	}
	_, b := s.request(alice, http.MethodGet, "/midgard/api/v1/devices", "")
	var out types.DevicesOutput
	json.Unmarshal(b, &out)
	for _, d := range out.Devices {
		if d.ID == old.e.ID && d.Counted {
			t.Fatalf("the forgotten device still counts: %+v", d)
		}
	}
}

// raw connects to the websocket as a device without speaking for it.
func raw(t *testing.T, s *server, p person) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(s.url(), "http") + "/midgard/api/v1/ws"
	c, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Authorization": {p.header()}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func sendFrame(t *testing.T, c *websocket.Conn, f wire.Frame) {
	t.Helper()
	b, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	c.WriteMessage(websocket.BinaryMessage, b)
}

// TestHelloFirst: a connection that does not say hello first is closed.
func TestHelloFirst(t *testing.T) {
	s := twoPeople(t)
	c := raw(t, s, alice)
	sendFrame(t, c, wire.NewCopy("text", []byte("no hello")))
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("the server answered a connection without a hello")
	}
	c = raw(t, s, alice)
	sendFrame(t, c, wire.Frame{Envelope: wire.Envelope{Type: wire.Hello, V: 99, Device: "future"}})
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := c.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseProtocolError) {
		t.Fatalf("another version: %v, want it closed", err)
	}
}

// TestSendingAgainNumbersOnce: a device sends what waits in its outbox again
// after reconnecting; what was numbered already is not numbered again.
func TestSendingAgainNumbersOnce(t *testing.T) {
	s := twoPeople(t)
	c := raw(t, s, alice)
	sendFrame(t, c, wire.Frame{Envelope: wire.Envelope{Type: wire.Hello, V: wire.Version, Device: "d", Name: "d", Clock: time.Now().UnixMilli()}})
	f := wire.NewCopy("text", []byte("once"))
	f.Ref = "r1"
	sendFrame(t, c, f)
	sendFrame(t, c, f)
	eventually(t, "it is numbered", func() bool { return len(s.queue(alice).Queue) >= 1 })
	time.Sleep(100 * time.Millisecond)
	if q := s.queue(alice); len(q.Queue) != 1 || q.Head != 1 {
		t.Fatalf("queue %+v, want one copy, numbered 1", q)
	}
}

// TestServerDropsSilentDevices: a device that vanished without closing its
// connection stayed listed forever, and written to by every broadcast.
func TestServerDropsSilentDevices(t *testing.T) {
	resetBlocklist(t)
	m := testMidgard(t)
	m.keepalive = keepalive{ping: 50 * time.Millisecond, wait: 300 * time.Millisecond, write: time.Second}
	s := newServer(t, m)
	c := raw(t, s, person{testUser, testEmail})
	sendFrame(t, c, wire.Frame{Envelope: wire.Envelope{Type: wire.Hello, V: wire.Version, Device: "silent", Clock: time.Now().UnixMilli()}})
	// it reads nothing, and so answers no ping
	online := func() bool {
		_, b := s.request(person{testUser, testEmail}, http.MethodGet, "/midgard/api/v1/devices", "")
		return strings.Contains(string(b), `"online":true`)
	}
	eventually(t, "the device is online", online)
	eventually(t, "the server dropped it", func() bool { return !online() })
}
