// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package device keeps a device in sync with its person's other devices,
// through the server (specs/redesign.md §3, §6). It is the part of the daemon,
// and of the tray app, that is not the clipboard: it stays connected, sends
// what happens on the device, applies what the server numbers to the
// device's history, and answers the server's questions from it.
//
// Once its person has a key (§11), it seals every copy it sends, and every
// answer, as it writes them, and opens every copy it receives before it
// keeps it: its history stays in the clear, on the device.
package device

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"changkun.de/x/midgard/internal/e2e"
	"changkun.de/x/midgard/internal/history"
	"changkun.de/x/midgard/internal/wire"
	"github.com/gorilla/websocket"
)

// Keepalive is how the connection to the server is kept alive.
type Keepalive struct {
	// Ping is how often the device pings the server. A connection that
	// stops answering — a network change, a VPN toggled, a laptop waking
	// up — would otherwise look open forever (#33).
	Ping time.Duration
	// Wait is how long the connection may stay silent before the device
	// gives up on it and reconnects. Any frame, ping, or pong counts.
	Wait time.Duration
	// Write bounds a single write.
	Write time.Duration
	// RetryMin and RetryMax bound the wait between reconnection attempts,
	// which doubles after each failure.
	RetryMin, RetryMax time.Duration
	// Settle is how long the device waits after an event for more before
	// it acknowledges what it has and updates the clipboard, so catching
	// up on many events acknowledges once and does not flicker.
	Settle time.Duration
}

// DefaultKeepalive is what a device uses.
var DefaultKeepalive = Keepalive{
	Ping:     30 * time.Second,
	Wait:     90 * time.Second,
	Write:    10 * time.Second,
	RetryMin: time.Second,
	RetryMax: time.Minute,
	Settle:   100 * time.Millisecond,
}

// Engine is one device's sync.
type Engine struct {
	ID      string         // the install's id, which the server knows it by
	Name    string         // its name, to show
	History *history.Store // its copy of its person's history
	// Dial connects to the server's websocket.
	Dial func(ctx context.Context) (*websocket.Conn, error)
	// Changed is called when a copy from elsewhere becomes the newest: it
	// belongs on the device's clipboard. A delete never calls it: removing
	// a copy from the history does not change what anyone has on their
	// clipboard.
	Changed   func(history.Entry)
	Keepalive Keepalive

	// Key is the person's key (§11), nil until the device has it; Since is
	// the last seq numbered before it was made. LoadKey, when set, is asked
	// for it again while the device must pair, as pairing may happen
	// elsewhere (mg pair); SaveKey, when set, keeps a key the device makes,
	// as its person's first. Without SaveKey, the device makes none.
	Key     *e2e.Key
	Since   uint64
	LoadKey func() (*e2e.Key, uint64, error)
	SaveKey func(*e2e.Key, uint64) error

	mu      sync.Mutex
	send    chan wire.Frame // the connection's, while there is one
	applied map[uint64]bool // copies applied since the clipboard was last looked at
	settle  *time.Timer
	sealing bool          // the connection seals: its person has a key, and this device it
	pairing bool          // its person has a key this device lacks
	kid     string        // the id of its person's key, as the server said
	rekey   chan struct{} // a key came, by pairing: connect again now
}

// Errors connecting ends in, on the way to sealing (§11).
var (
	errNewKey       = errors.New("made this person's key; saying hello with it")
	errNeedsPairing = errors.New("this device's person has a key it lacks: pair it with one of their devices")
)

// NeedsPairing reports whether the device's person has a key it lacks, and
// the id of that key: until it pairs, it syncs nothing.
func (e *Engine) NeedsPairing() (bool, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pairing, e.kid
}

// Sealing reports whether the device seals what it sends: its person has a
// key, and it has it too.
func (e *Engine) Sealing() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sealing
}

// PersonKey is the key the device has, and since, to pair another with;
// nil when it has none.
func (e *Engine) PersonKey() (*e2e.Key, uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.Key, e.Since
}

// SetKey gives the device its person's key, as pairing did: it keeps it, and
// connects again with it at once.
func (e *Engine) SetKey(k *e2e.Key, since uint64) error {
	if e.SaveKey != nil {
		if err := e.SaveKey(k, since); err != nil {
			return err
		}
	}
	e.mu.Lock()
	e.Key, e.Since, e.pairing = k, since, false
	rekey := e.rekey
	e.mu.Unlock()
	if rekey != nil {
		select {
		case rekey <- struct{}{}:
		default:
		}
	}
	return nil
}

// Online reports whether the device is connected to the server.
func (e *Engine) Online() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.send != nil
}

// Copy records a copy made on the device, and sends it when it can.
func (e *Engine) Copy(ctx context.Context, mime string, data []byte) error {
	return e.add(ctx, wire.NewCopy(mime, data))
}

// Delete removes the copy numbered seq from the history, here at once and
// on every device once the server has it.
func (e *Engine) Delete(ctx context.Context, seq uint64) error {
	return e.add(ctx, wire.Frame{Envelope: wire.Envelope{Type: wire.Delete, Target: seq}})
}

// Clear removes every copy from the history, the same way.
func (e *Engine) Clear(ctx context.Context) error {
	return e.add(ctx, wire.Frame{Envelope: wire.Envelope{Type: wire.Clear}})
}

// add puts f in the outbox, and on the connection if there is one. Both
// happen under e.mu, as connecting reads the outbox under it: a frame is in
// the outbox connecting sends, or on the connection, never both or neither.
func (e *Engine) add(ctx context.Context, f wire.Frame) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	f, err := e.History.Add(ctx, f, e.send == nil)
	if err != nil {
		return err
	}
	if e.send != nil {
		e.enqueue(f)
	}
	return nil
}

// enqueue sends f on the connection, or drops it if the connection is too
// far behind: it will be sent from the outbox on the next one. e.mu is held.
func (e *Engine) enqueue(f wire.Frame) {
	select {
	case e.send <- f:
	default:
		slog.Warn("the connection to the server is behind; sending later", "ref", f.Ref)
	}
}

// Run keeps the device connected and in sync until ctx is done: it connects,
// serves the connection until it fails, and connects again, waiting longer
// after each failed attempt.
func (e *Engine) Run(ctx context.Context) {
	ka := e.Keepalive
	wait := ka.RetryMin
	e.mu.Lock()
	if e.rekey == nil {
		e.rekey = make(chan struct{}, 1)
	}
	rekey := e.rekey
	e.mu.Unlock()
	for ctx.Err() == nil {
		conn, err := e.connect(ctx)
		switch {
		case errors.Is(err, errNewKey):
			slog.Info("made this person's key, the first of their devices to seal")
			continue
		case errors.Is(err, errNeedsPairing):
			// until it pairs, here or through mg pair, it asks again now and
			// then, and at once when SetKey gives it the key
			slog.Warn(err.Error())
			select {
			case <-ctx.Done():
				return
			case <-rekey:
			case <-time.After(ka.RetryMax):
			}
			continue
		case err != nil:
			slog.Error("cannot connect to the midgard server", "err", err, "retry_in", wait)
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			wait = min(wait*2, ka.RetryMax)
			continue
		}
		slog.Info("connected to the midgard server", "device", e.Name)

		start := time.Now()
		err = e.serve(ctx, conn)
		if ctx.Err() != nil {
			return
		}
		slog.Error("lost the connection to the midgard server, reconnecting", "err", err)
		if time.Since(start) >= ka.RetryMax {
			wait = ka.RetryMin // it was up for a while; try again straight away
			continue
		}
		// A connection that dies as soon as it is made counts as a failed
		// attempt, or a server that accepts and drops would be redialed
		// in a tight loop.
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, ka.RetryMax)
	}
}

// connect dials the server and says hello: which device this is, what of
// the history it has, and which key. The welcome says the person's key: the
// device seals with it, makes it when it is the person's first device to
// seal, or must pair when it lacks it (§11).
func (e *Engine) connect(ctx context.Context) (*websocket.Conn, error) {
	e.mu.Lock()
	reload := e.LoadKey != nil && (e.Key == nil || e.pairing)
	e.mu.Unlock()
	if reload {
		if k, since, err := e.LoadKey(); err != nil {
			slog.Error("cannot read this device's key", "err", err)
		} else if k != nil {
			e.mu.Lock()
			e.Key, e.Since = k, since
			e.mu.Unlock()
		}
	}
	e.mu.Lock()
	key := e.Key
	e.mu.Unlock()
	kid := ""
	if key != nil {
		kid = key.ID()
	}

	conn, err := e.Dial(ctx)
	if err != nil {
		return nil, err
	}
	acked, err := e.History.Acked(ctx)
	if err != nil {
		conn.Close()
		return nil, err
	}
	gaps, err := e.History.Gaps(ctx)
	if err != nil {
		conn.Close()
		return nil, err
	}
	conn.SetReadLimit(wire.MaxPayload + 1<<16)
	conn.SetWriteDeadline(time.Now().Add(e.Keepalive.Write))
	conn.SetReadDeadline(time.Now().Add(e.Keepalive.Wait))
	if err := write(conn, wire.Frame{Envelope: wire.Envelope{
		Type: wire.Hello, V: wire.Version, Device: e.ID, Name: e.Name,
		Clock: time.Now().UnixMilli(), Acked: acked, Gaps: gaps, Kid: kid,
	}}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("cannot say hello: %w", err)
	}
	_, b, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("no answer to hello: %w", err)
	}
	f, err := wire.Unmarshal(b)
	if err != nil || f.Type != wire.Welcome {
		conn.Close()
		return nil, fmt.Errorf("the server did not welcome the device: %q", b)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.kid = f.Kid
	switch {
	case f.V < 2:
		// a server from before encryption: nothing is sealed
		e.sealing, e.pairing = false, false
	case f.Kid == "" && key == nil && e.SaveKey != nil:
		// The person has no key, and this is their first device to seal: it
		// makes their key, and says hello again with it, which registers
		// it. Before it, the history is in the clear.
		k, err := e2e.NewKey()
		if err == nil {
			err = e.SaveKey(k, acked)
		}
		conn.Close()
		if err != nil {
			return nil, fmt.Errorf("cannot make this person's key: %w", err)
		}
		e.Key, e.Since = k, acked
		return nil, errNewKey
	case f.Kid != "" && f.Kid != kid:
		e.sealing, e.pairing = false, true
		conn.Close()
		return nil, errNeedsPairing
	default:
		// the person's key is this device's, or no one has one yet
		e.sealing, e.pairing = f.Kid != "", false
	}
	return conn, nil
}

// serve runs one connection until it fails or ctx is done. This goroutine is
// the connection's only writer and a second one its only reader, which is
// what gorilla/websocket allows.
func (e *Engine) serve(ctx context.Context, conn *websocket.Conn) error {
	defer conn.Close()
	ka := e.Keepalive

	alive := func() { conn.SetReadDeadline(time.Now().Add(ka.Wait)) }
	alive()
	conn.SetPongHandler(func(string) error { alive(); return nil })
	conn.SetPingHandler(func(data string) error {
		alive()
		err := conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(ka.Write))
		if errors.Is(err, websocket.ErrCloseSent) {
			return nil
		}
		return err
	})

	// From here on what happens on the device goes on this connection;
	// what happened before it waits in the outbox, sent first.
	send := make(chan wire.Frame, 256)
	e.mu.Lock()
	outbox, err := e.History.Outbox(ctx)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	e.send = send
	var key *e2e.Key // what this connection seals with and opens with
	if e.sealing {
		key = e.Key
	}
	since := e.Since
	e.mu.Unlock()
	write := func(conn *websocket.Conn, f wire.Frame) error {
		f, err := seal(key, f)
		if err != nil {
			return err
		}
		return write(conn, f)
	}
	defer func() {
		e.mu.Lock()
		e.send = nil
		e.mu.Unlock()
	}()

	readErr := make(chan error, 1)
	go func() { readErr <- e.readFrom(ctx, conn, send, alive, key, since) }()

	for _, f := range outbox {
		conn.SetWriteDeadline(time.Now().Add(ka.Write))
		if err := write(conn, f); err != nil {
			return fmt.Errorf("cannot send what waited: %w", err)
		}
	}

	ping := time.NewTicker(ka.Ping)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(ka.Write))
			return ctx.Err()
		case err := <-readErr:
			return err
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(ka.Write)); err != nil {
				return fmt.Errorf("cannot ping the server: %w", err)
			}
		case f := <-send:
			conn.SetWriteDeadline(time.Now().Add(ka.Write))
			if err := write(conn, f); err != nil {
				// A copy lost with the connection is still in the
				// outbox, and goes on the next one.
				return fmt.Errorf("cannot write to the server: %w", err)
			}
		}
	}
}

// readFrom applies what the server sends, and answers what it asks, until
// the connection fails.
func (e *Engine) readFrom(ctx context.Context, conn *websocket.Conn, send chan<- wire.Frame, alive func(), key *e2e.Key, since uint64) error {
	for {
		_, b, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		alive()
		f, err := wire.Unmarshal(b)
		if err != nil {
			slog.Error("cannot read a frame from the server", "err", err)
			continue
		}
		switch f.Type {
		case wire.Event:
			f = open(key, since, f)
			applied, ours, err := e.History.Apply(ctx, f)
			if err != nil {
				slog.Error("cannot apply an event", "seq", f.Seq, "err", err)
				continue
			}
			if applied {
				e.applied1(f, ours)
			}
		case wire.Want:
			for _, a := range e.answer(ctx, f) {
				select {
				case send <- a:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
}

// applied1 notes that f was applied, and settles once no more follow. The
// device's own copy, come back numbered, is on its clipboard already.
func (e *Engine) applied1(f wire.Frame, ours bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if f.Kind == wire.KindCopy && !ours {
		if e.applied == nil {
			e.applied = map[uint64]bool{}
		}
		e.applied[f.Seq] = true
	}
	if e.settle != nil {
		e.settle.Stop()
	}
	e.settle = time.AfterFunc(e.Keepalive.Settle, e.settled)
}

// settled acknowledges what the device has, and puts the newest copy on the
// clipboard if it is one that just arrived.
func (e *Engine) settled() {
	ctx := context.Background()
	e.mu.Lock()
	applied := e.applied
	e.applied = nil
	send := e.send
	e.mu.Unlock()

	acked, err := e.History.Acked(ctx)
	if err != nil {
		slog.Error("cannot read the history", "err", err)
		return
	}
	gaps, err := e.History.Gaps(ctx)
	if err != nil {
		slog.Error("cannot read the history", "err", err)
		return
	}
	if send != nil {
		e.mu.Lock()
		if e.send == send {
			e.enqueue(wire.Frame{Envelope: wire.Envelope{Type: wire.Ack, Acked: acked, Gaps: gaps}})
		}
		e.mu.Unlock()
	}

	newest, ok, err := e.History.Newest(ctx, true)
	if err != nil || !ok || newest.Waiting() || !applied[newest.Seq] || e.Changed == nil {
		return
	}
	e.Changed(newest)
}

// answer is the device's answer to a want: the events or copies asked for,
// and done.
func (e *Engine) answer(ctx context.Context, want wire.Frame) []wire.Frame {
	var out []wire.Frame
	var err error
	switch {
	case want.Span != nil:
		var evs []wire.Frame
		evs, err = e.History.Events(ctx, *want.Span)
		out = evs
	case want.Newest:
		var n history.Entry
		var ok bool
		n, ok, err = e.History.Newest(ctx, false)
		if ok {
			out = []wire.Frame{n.Frame()}
		}
	case want.List > 0:
		var list []history.Entry
		list, err = e.History.Previews(ctx, want.List, want.Preview)
		for _, c := range list {
			if c.Waiting() {
				continue
			}
			f := c.Frame() // its payload, if any, is a preview
			f.Bare = true
			out = append(out, f)
		}
	}
	for i := range out {
		out[i].Type, out[i].ID = wire.Have, want.ID
	}
	done := wire.Frame{Envelope: wire.Envelope{Type: wire.Done, ID: want.ID}}
	if err != nil {
		done.Err = err.Error()
	}
	return append(out, done)
}

// seal seals the bytes of a copy the device sends, or answers with, when the
// connection seals: a copy's as a copy, a preview's as a preview (§11).
func seal(key *e2e.Key, f wire.Frame) (wire.Frame, error) {
	if key == nil || len(f.Payload) == 0 || f.Kid != "" || f.Kind != wire.KindCopy {
		return f, nil
	}
	kind := e2e.Copy
	if f.Bare {
		kind = e2e.Preview
	}
	sealed, err := key.Seal(kind, f.Formats, f.Payload)
	if err != nil {
		return f, err
	}
	f.Payload, f.Kid = sealed, key.ID()
	return f, nil
}

// open opens a copy the server sent, when the connection seals. One the
// device cannot open, and one in the clear numbered after its person's key
// was made, cannot be theirs: the device keeps it as a void, a number that
// holds nothing, so its history stays whole (§11).
func open(key *e2e.Key, since uint64, f wire.Frame) wire.Frame {
	if f.Kind != wire.KindCopy {
		return f
	}
	void := wire.Frame{Envelope: wire.Envelope{Type: wire.Event, Kind: wire.KindVoid, Seq: f.Seq, Time: f.Time, Origin: f.Origin, Ref: f.Ref}}
	switch {
	case f.Kid != "":
		if key == nil || f.Kid != key.ID() {
			slog.Error("a copy sealed with another key; keeping it as nothing", "seq", f.Seq)
			return void
		}
		plain, err := key.Open(e2e.Copy, f.Formats, f.Payload)
		if err != nil {
			slog.Error("a copy that does not open; keeping it as nothing", "seq", f.Seq, "err", err)
			return void
		}
		f.Payload, f.Kid = plain, ""
		if len(plain) == 0 {
			f.Payload = nil
		}
	case key != nil && f.Seq > since:
		slog.Error("a copy in the clear, numbered after the key was made; keeping it as nothing", "seq", f.Seq)
		return void
	}
	return f
}

func write(conn *websocket.Conn, f wire.Frame) error {
	b, err := f.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, b)
}
