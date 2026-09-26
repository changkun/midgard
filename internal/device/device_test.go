// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package device

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/history"
	"changkun.de/x/midgard/internal/wire"
	"github.com/gorilla/websocket"
)

// fast are connection timings short enough for a test.
var fast = Keepalive{
	Ping: 50 * time.Millisecond, Wait: 300 * time.Millisecond, Write: time.Second,
	RetryMin: 10 * time.Millisecond, RetryMax: 50 * time.Millisecond, Settle: 10 * time.Millisecond,
}

// fakeServer welcomes each device that says hello, and hands the
// connection, numbered from 1, with the hello, to session.
func fakeServer(t *testing.T, session func(n int64, hello wire.Frame, c *websocket.Conn)) (url string, conns *atomic.Int64) {
	t.Helper()
	conns = new(atomic.Int64)
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		_, b, err := c.ReadMessage()
		if err != nil {
			return
		}
		hello, err := wire.Unmarshal(b)
		if err != nil || hello.Type != wire.Hello {
			t.Errorf("the device began with %q", b)
			return
		}
		send(c, wire.Frame{Envelope: wire.Envelope{Type: wire.Welcome}})
		session(conns.Add(1), hello, c)
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http"), conns
}

func send(c *websocket.Conn, f wire.Frame) {
	b, _ := f.Marshal()
	c.WriteMessage(websocket.BinaryMessage, b)
}

// next is the next frame from the device, skipping acks.
func next(c *websocket.Conn) (wire.Frame, error) {
	for {
		_, b, err := c.ReadMessage()
		if err != nil {
			return wire.Frame{}, err
		}
		f, err := wire.Unmarshal(b)
		if err != nil || f.Type == wire.Ack {
			continue
		}
		return f, nil
	}
}

func engine(t *testing.T, url string) *Engine {
	t.Helper()
	h, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return &Engine{
		ID: "test", Name: "test", History: h, Keepalive: fast,
		Dial: func(ctx context.Context) (*websocket.Conn, error) {
			c, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
			return c, err
		},
	}
}

func run(t *testing.T, e *Engine) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); e.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
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
	url, conns := fakeServer(t, func(n int64, _ wire.Frame, c *websocket.Conn) {
		if n <= 3 {
			return // drop the connection straight after the welcome
		}
		for {
			f, err := next(c)
			if err != nil {
				return
			}
			if f.Type == wire.Copy {
				got <- string(f.Payload)
				return
			}
		}
	})
	e := engine(t, url)
	run(t, e)

	eventually(t, "the device reconnected three times", func() bool { return conns.Load() >= 4 && e.Online() })
	e.Copy(context.Background(), "text", []byte("after"))
	select {
	case d := <-got:
		if d != "after" {
			t.Fatalf("server got %q, want %q", d, "after")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a copy after reconnecting never reached the server")
	}
}

// TestReconnectsFromASilentServer covers the connection that stays open but
// carries nothing, which is what a network change leaves behind: nothing was
// ever read from it, so the daemon waited on it forever.
func TestReconnectsFromASilentServer(t *testing.T) {
	url, conns := fakeServer(t, func(n int64, _ wire.Frame, c *websocket.Conn) {
		if n == 1 {
			time.Sleep(3 * time.Second) // say nothing and answer no ping, without closing
			return
		}
		for {
			if _, err := next(c); err != nil {
				return
			}
		}
	})
	run(t, engine(t, url))
	eventually(t, "the device gave up on the silent connection", func() bool { return conns.Load() >= 2 })
}

// TestOutbox: what happened while the device was offline is sent when it
// connects, marked offline, and leaves the outbox once numbered.
func TestOutbox(t *testing.T) {
	got := make(chan wire.Frame, 4)
	url, _ := fakeServer(t, func(_ int64, _ wire.Frame, c *websocket.Conn) {
		for {
			f, err := next(c)
			if err != nil {
				return
			}
			got <- f
			if f.Type == wire.Copy {
				// number it and send it back
				f.Type, f.Seq, f.Time = wire.Event, 1, time.Now().UnixMilli()
				send(c, f)
			}
		}
	})
	e := engine(t, url)
	ctx := context.Background()
	if err := e.Copy(ctx, "text", []byte("made offline")); err != nil {
		t.Fatal(err)
	}
	run(t, e)

	select {
	case f := <-got:
		if f.Type != wire.Copy || !f.Offline || f.Ref == "" || string(f.Payload) != "made offline" {
			t.Fatalf("the server got %+v", f.Envelope)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the outbox was not sent")
	}
	eventually(t, "the outbox is empty", func() bool {
		out, _ := e.History.Outbox(ctx)
		return len(out) == 0
	})
	// a copy made online is sent at once, not marked offline
	e.Copy(ctx, "text", []byte("made online"))
	select {
	case f := <-got:
		if f.Offline || string(f.Payload) != "made online" {
			t.Fatalf("the server got %+v", f.Envelope)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the copy was not sent")
	}
}

// TestHelloAndAnswers: the device says what it has, and answers what the
// server asks from its history.
func TestHelloAndAnswers(t *testing.T) {
	hellos := make(chan wire.Frame, 1)
	answers := make(chan []wire.Frame, 3)
	url, _ := fakeServer(t, func(_ int64, hello wire.Frame, c *websocket.Conn) {
		hellos <- hello
		for _, w := range []wire.Envelope{
			{Type: wire.Want, ID: "span", Span: &wire.Span{From: 1, To: 2}},
			{Type: wire.Want, ID: "newest", Newest: true},
			{Type: wire.Want, ID: "list", List: 10},
		} {
			send(c, wire.Frame{Envelope: w})
			var got []wire.Frame
			for {
				f, err := next(c)
				if err != nil {
					return
				}
				if f.ID != w.ID {
					t.Errorf("an answer to %q for %q", f.ID, w.ID)
				}
				got = append(got, f)
				if f.Type == wire.Done {
					break
				}
			}
			answers <- got
		}
		for {
			if _, err := next(c); err != nil {
				return
			}
		}
	})
	e := engine(t, url)
	ctx := context.Background()
	for _, seq := range []uint64{1, 2, 4} {
		f := wire.NewCopy("text", []byte{'a' + byte(seq)})
		f.Type, f.Seq, f.Time = wire.Event, seq, time.Now().Add(time.Duration(seq)*time.Second).UnixMilli()
		e.History.Apply(ctx, f)
	}
	run(t, e)

	hello := <-hellos
	if hello.Device != "test" || hello.V != wire.Version || hello.Acked != 4 || len(hello.Gaps) != 1 || hello.Gaps[0] != (wire.Span{From: 3, To: 3}) || hello.Clock == 0 {
		t.Fatalf("hello %+v", hello.Envelope)
	}
	span := <-answers
	if len(span) != 3 || span[0].Seq != 1 || span[1].Seq != 2 || string(span[1].Payload) != "c" || span[2].Type != wire.Done {
		t.Fatalf("the answer to a span: %+v", span)
	}
	newest := <-answers
	if len(newest) != 2 || newest[0].Seq != 4 || string(newest[0].Payload) != "e" {
		t.Fatalf("the answer to newest: %+v", newest)
	}
	list := <-answers
	if len(list) != 4 || list[0].Seq != 4 || !list[0].Bare || list[0].Payload != nil || list[0].Size() != 1 {
		t.Fatalf("the answer to a list: %+v", list)
	}
}
