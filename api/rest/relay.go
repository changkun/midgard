// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"changkun.de/x/midgard/internal/store"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/wire"
)

// The relay passes each person's copies between their devices, numbers
// everything that happens to their history, and holds each copy in memory
// until every one of their devices has it (specs/redesign.md §3, §6). It
// writes no copy to disk: what it keeps is the last number it gave out and
// the devices, in the store.

// holdLimits bound what the relay holds for a person, and how long it waits.
type holdLimits struct {
	bytes  int           // the copies held for one person, at most
	age    time.Duration // how long a copy is held, at most
	forget time.Duration // a device unseen this long no longer counts
	answer time.Duration // how long a device may take to answer
}

var defaultHold = holdLimits{bytes: 64 << 20, age: 7 * 24 * time.Hour, forget: 30 * 24 * time.Hour, answer: 5 * time.Second}

// Errors a read from a person's devices can end in.
var (
	errNoDevice = errors.New("none of your devices is online to ask")
	errNoAnswer = errors.New("your device did not answer in time")
)

// Errors a copy is refused with, once its person has a key or when it is
// sealed under one they have not (§11).
var (
	errNotSealed = errors.New("your copies are encrypted: this one is not; update this client, or pair it")
	errOtherKey  = errors.New("sealed with a key that is not your devices': pair this client again")
	errNoKey     = errors.New("sealed with a key your devices do not have: pair a device first")
)

// What a device's hello can be refused with (§11).
var (
	errUpdate = errors.New("your devices encrypt their copies now: update midgard on this one")
	errPair   = errors.New("this device's key is not your devices': pair it with one of them")
)

type relay struct {
	store *store.Store
	hold  holdLimits
	now   func() time.Time

	mu    sync.Mutex
	rooms map[string]*room

	pairs *mailboxes // pairing boxes waiting for new devices (pair.go)
}

func newRelay(s *store.Store) *relay {
	return &relay{store: s, hold: defaultHold, now: time.Now, rooms: map[string]*room{}, pairs: &mailboxes{now: time.Now}}
}

// room is one person's: their devices online, and what is held for them. A
// frame never leaves its room (§5).
type room struct {
	owner string
	mu    sync.Mutex
	conns map[string]*link         // online, by device id
	known map[string]*store.Device // every device, from the store
	held  []held                   // in order of seq
	size  int                      // the bytes held
	kid   string                   // the id of the person's key; "" while they have none (§11)
	asks  map[string]*ask          // wants waiting for an answer, by id
	fills map[string]bool          // devices a peer is filling in, by id
}

// held is an event the relay holds, with when it came and which device it
// came from.
type held struct {
	wire.Frame
	at     time.Time
	device string
}

// link is one device's connection.
type link struct {
	id, name string
	v        int           // the protocol it speaks: under 2, it seals nothing
	out      chan []byte   // the connection's writer sends these
	gone     chan struct{} // closed when the connection is over
	once     sync.Once
	offset   time.Duration // the server's clock minus the device's
}

func (l *link) close() { l.once.Do(func() { close(l.gone) }) }

// send queues b for the device. A device that cannot keep up is dropped:
// it catches up when it connects again.
func (l *link) send(b []byte) {
	select {
	case l.out <- b:
	case <-l.gone:
	default:
		slog.Warn("a device is too far behind; dropping its connection", "device", l.name)
		l.close()
	}
}

// ask is a want waiting for its answer.
type ask struct {
	device string           // the device asked
	have   func(wire.Frame) // called with each answer; the room is locked
	done   chan string      // closed, or given the device's error, at the end
}

// room is owner's room, created on first use.
func (r *relay) room(ctx context.Context, owner string) (*room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rm, ok := r.rooms[owner]; ok {
		return rm, nil
	}
	devices, err := r.store.Devices(ctx, owner)
	if err != nil {
		return nil, err
	}
	kid, err := r.store.Kid(ctx, owner)
	if err != nil {
		return nil, err
	}
	rm := &room{
		owner: owner, conns: map[string]*link{}, known: map[string]*store.Device{},
		asks: map[string]*ask{}, fills: map[string]bool{}, kid: kid,
	}
	for _, d := range devices {
		rm.known[d.ID] = &d
	}
	r.rooms[owner] = rm
	return rm, nil
}

// admit decides whether a device that said hello may join (§11). One that
// seals, while its person has no key, registers the key it has as theirs:
// the first device to do so wins, and devices that cannot seal are sent
// away, before they take a sealed copy for text. One whose key is not its
// person's must pair; one that cannot seal is let in only while its person
// has no key. It returns the person's key id.
func (r *relay) admit(ctx context.Context, rm *room, hello wire.Frame) (string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if hello.V < 2 {
		if rm.kid != "" {
			return rm.kid, errUpdate
		}
		return "", nil
	}
	if rm.kid == "" && hello.Kid != "" {
		kid, err := r.store.SetKid(ctx, rm.owner, hello.Kid)
		if err != nil {
			return "", err
		}
		rm.kid = kid
		for _, l := range rm.conns {
			if l.v < 2 {
				l.close()
			}
		}
	}
	if rm.kid != "" && hello.Kid != rm.kid {
		return rm.kid, errPair
	}
	return rm.kid, nil
}

// join lets a device in, having said hello, and sends it what it lacks of
// what is held: the rest, other devices fill in (fill).
func (r *relay) join(ctx context.Context, rm *room, l *link, hello wire.Frame) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if old, ok := rm.conns[l.id]; ok {
		old.close() // the same install connected again; the new connection wins
	}
	rm.conns[l.id] = l
	d := &store.Device{ID: l.id, Name: l.name, LastSeen: r.now(), Acked: hello.Acked, Gaps: hello.Gaps}
	rm.known[l.id] = d
	if err := r.store.SeeDevice(ctx, rm.owner, *d); err != nil {
		return err
	}
	head, err := r.store.Head(ctx, rm.owner)
	if err != nil {
		return err
	}
	l.send(encode(wire.Frame{Envelope: wire.Envelope{Type: wire.Welcome, V: wire.Version, Head: head, Kid: rm.kid}}))
	for _, h := range rm.held {
		if lacks(d, h.Seq) {
			l.send(encode(h.Frame))
		}
	}
	return nil
}

// leave takes the device's connection out of the room, if it is still its
// current one.
func (r *relay) leave(ctx context.Context, rm *room, l *link) {
	l.close()
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if rm.conns[l.id] != l {
		return
	}
	delete(rm.conns, l.id)
	for id, a := range rm.asks {
		if a.device == l.id {
			delete(rm.asks, id)
			close(a.done)
		}
	}
	if d := rm.known[l.id]; d != nil {
		d.LastSeen = r.now()
		r.store.SeeDevice(ctx, rm.owner, *d)
	}
}

// lacks reports whether device d lacks the event numbered seq.
func lacks(d *store.Device, seq uint64) bool {
	if seq > d.Acked {
		return true
	}
	for _, g := range d.Gaps {
		if g.From <= seq && seq <= g.To {
			return true
		}
	}
	return false
}

// number gives what happened the next number in the person's history,
// holds it, and sends it to each of their devices online, the one it came
// from included, which learns its number by its ref. from is the device it
// came from, or nil for the web page, a Shortcut or mg; origin names it.
func (r *relay) number(ctx context.Context, rm *room, from *link, origin string, f wire.Frame) (wire.Frame, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	now := r.now()

	// A device sends what waits in its outbox again when it reconnects;
	// what the relay already numbered, it sends back instead.
	if from != nil && f.Ref != "" {
		for _, h := range rm.held {
			if h.device == from.id && h.Ref == f.Ref {
				from.send(encode(h.Frame))
				return h.Frame, nil
			}
		}
	}

	// A copy is sealed with its person's key once they have one, and not
	// before (§11). The relay cannot tell ciphertext from text; it goes by
	// what the frame says, and a device refuses a copy that lies.
	if f.Kind == wire.KindCopy && f.Kid != rm.kid {
		switch {
		case rm.kid == "":
			return wire.Frame{}, errNoKey
		case f.Kid == "":
			return wire.Frame{}, errNotSealed
		default:
			return wire.Frame{}, errOtherKey
		}
	}

	seq, err := r.store.NextSeq(ctx, rm.owner)
	if err != nil {
		return wire.Frame{}, err
	}
	ev := wire.Frame{
		Envelope: wire.Envelope{
			Type: wire.Event, Seq: seq, Kind: f.Kind, Time: now.UnixMilli(),
			Origin: origin, Ref: f.Ref, Formats: f.Formats, Kid: f.Kid,
		},
		Payload: f.Payload,
	}
	switch f.Kind {
	case wire.KindCopy:
		// A copy made offline is placed at when it was made, by the
		// device's clock set right, and never later than now (§6).
		if f.Offline && from != nil && f.Time > 0 {
			ev.Time = min(time.UnixMilli(f.Time).Add(from.offset).UnixMilli(), now.UnixMilli())
		}
	case wire.KindDelete:
		ev.Target = f.Target
		rm.void(func(h *held) bool { return h.Seq == f.Target })
	case wire.KindClear:
		ev.Target = seq - 1 // what the server has numbered, which may be more than the device had
		rm.void(func(h *held) bool { return h.Seq < seq })
	default:
		return wire.Frame{}, errors.New("nothing to number")
	}
	device := ""
	if from != nil {
		device = from.id
	}
	rm.held = append(rm.held, held{Frame: ev, at: now, device: device})
	rm.size += len(ev.Payload)
	b := encode(ev)
	for _, l := range rm.conns {
		l.send(b)
	}
	r.evict(rm)
	return ev, nil
}

// void turns the held copies that match into voids: the copy is gone, its
// number stays, so a device catching up learns it is gone rather than
// missing. The room is locked.
func (rm *room) void(match func(*held) bool) {
	for i := range rm.held {
		h := &rm.held[i]
		if h.Kind == wire.KindCopy && match(h) {
			rm.size -= len(h.Payload)
			h.Frame = wire.Frame{Envelope: wire.Envelope{Type: wire.Event, Seq: h.Seq, Kind: wire.KindVoid, Time: h.Time, Origin: h.Origin}}
		}
	}
}

// ack records what a device has, and forgets what every device has.
func (r *relay) ack(ctx context.Context, rm *room, l *link, acked uint64, gaps []wire.Span) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	d := rm.known[l.id]
	if d == nil {
		return
	}
	d.Acked, d.Gaps, d.LastSeen = acked, gaps, r.now()
	if err := r.store.SeeDevice(ctx, rm.owner, *d); err != nil {
		slog.Error("cannot record what a device has", "err", err)
	}
	r.evict(rm)
}

// counted are the devices the relay holds copies for: those not forgotten
// and seen lately, and those online. The room is locked.
func (r *relay) counted(rm *room) []*store.Device {
	var out []*store.Device
	for id, d := range rm.known {
		_, online := rm.conns[id]
		if online || (!d.Forgotten && r.now().Sub(d.LastSeen) < r.hold.forget) {
			out = append(out, d)
		}
	}
	return out
}

// evict forgets what every counted device has, and past the limits, the
// oldest first. The room is locked.
func (r *relay) evict(rm *room) {
	devices := r.counted(rm)
	now := r.now()
	kept := rm.held[:0]
	size := rm.size
	for i, h := range rm.held {
		everyone := len(devices) > 0
		for _, d := range devices {
			if lacks(d, h.Seq) {
				everyone = false
				break
			}
		}
		// what is left to keep after h, if h were kept: the newest are
		// kept within the byte limit, the oldest go
		over := size > r.hold.bytes && i < len(rm.held)-1
		if everyone || now.Sub(h.at) > r.hold.age || over {
			size -= len(h.Payload)
			continue
		}
		kept = append(kept, h)
	}
	clear(rm.held[len(kept):])
	rm.held = kept
	rm.size = size
}

// fill asks the devices online for what the others lack and the relay no
// longer holds: after a restart, or once a copy's hold ran out.
func (r *relay) fill(ctx context.Context, rm *room) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	head, err := r.store.Head(ctx, rm.owner)
	if err != nil {
		return
	}
	for id, l := range rm.conns {
		if rm.fills[id] {
			continue
		}
		d := rm.known[id]
		spans := r.missing(rm, d, head)
		if len(spans) == 0 {
			continue
		}
		// the peer online that has the most
		var peer *link
		var best uint64
		for pid, p := range rm.conns {
			if pd := rm.known[pid]; pid != id && pd != nil && pd.Acked > best {
				peer, best = p, pd.Acked
			}
		}
		if peer == nil {
			continue
		}
		rm.fills[id] = true
		pending := 0
		finished := make(chan string, len(spans))
		for _, s := range spans {
			if s.From > best {
				continue
			}
			s.To = min(s.To, best)
			pending++
			a := &ask{device: peer.id, done: make(chan string, 1), have: func(f wire.Frame) {
				f.Type, f.ID = wire.Event, ""
				l.send(encode(f))
			}}
			want := r.want(rm, a, wire.Envelope{Span: &s})
			peer.send(encode(want))
			go func() {
				select {
				case <-a.done:
				case <-time.After(r.hold.answer * 6):
					rm.mu.Lock()
					delete(rm.asks, want.ID)
					rm.mu.Unlock()
				}
				finished <- ""
			}()
		}
		go func() {
			for range pending {
				<-finished
			}
			rm.mu.Lock()
			delete(rm.fills, id)
			rm.mu.Unlock()
		}()
	}
}

// missing are the spans below head that d lacks and the relay does not
// hold. The room is locked.
func (r *relay) missing(rm *room, d *store.Device, head uint64) []wire.Span {
	var lack []wire.Span
	if d.Acked < head {
		lack = append(lack, wire.Span{From: d.Acked + 1, To: head})
	}
	lack = append(lack, d.Gaps...)
	var out []wire.Span
	for _, s := range lack {
		var cur *wire.Span
		for seq := s.From; seq <= s.To; seq++ {
			if rm.holds(seq) {
				cur = nil
				continue
			}
			if cur == nil {
				out = append(out, wire.Span{From: seq, To: seq})
				cur = &out[len(out)-1]
			} else {
				cur.To = seq
			}
		}
	}
	return out
}

func (rm *room) holds(seq uint64) bool {
	_, ok := slices.BinarySearchFunc(rm.held, seq, func(h held, seq uint64) int { return cmp.Compare(h.Seq, seq) })
	return ok
}

// want registers a and returns the want frame to send. The room is locked.
func (r *relay) want(rm *room, a *ask, w wire.Envelope) wire.Frame {
	b := make([]byte, 8)
	rand.Read(b)
	w.Type, w.ID = wire.Want, hex.EncodeToString(b)
	rm.asks[w.ID] = a
	return wire.Frame{Envelope: w}
}

// answer passes a device's have or done to what asked.
func (r *relay) answer(rm *room, l *link, f wire.Frame) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	a, ok := rm.asks[f.ID]
	if !ok || a.device != l.id {
		return
	}
	if f.Type == wire.Done {
		delete(rm.asks, f.ID)
		if f.Err != "" {
			a.done <- f.Err
		}
		close(a.done)
		return
	}
	a.have(f)
}

// query asks the person's device online that has the most for what w
// wants, and waits for the answer.
func (r *relay) query(ctx context.Context, rm *room, w wire.Envelope) ([]wire.Frame, error) {
	var got []wire.Frame
	a := &ask{done: make(chan string, 1), have: func(f wire.Frame) { got = append(got, f) }}
	rm.mu.Lock()
	var dev *link
	var best uint64
	for id, l := range rm.conns {
		if d := rm.known[id]; dev == nil || (d != nil && d.Acked > best) {
			dev = l
			if d != nil {
				best = d.Acked
			}
		}
	}
	if dev == nil {
		rm.mu.Unlock()
		return nil, errNoDevice
	}
	a.device = dev.id
	want := r.want(rm, a, w)
	dev.send(encode(want))
	rm.mu.Unlock()

	select {
	case msg, ok := <-a.done:
		rm.mu.Lock()
		defer rm.mu.Unlock()
		if ok && msg != "" {
			return nil, errors.New(msg)
		}
		if !ok && got == nil {
			// the device went away without answering, or had nothing
			return nil, nil
		}
		return got, nil
	case <-time.After(r.hold.answer):
	case <-ctx.Done():
	}
	rm.mu.Lock()
	delete(rm.asks, want.ID)
	rm.mu.Unlock()
	return nil, errNoAnswer
}

// newest is the newest copy held, by (time, seq), if any.
func (rm *room) newest() (wire.Frame, bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	var n wire.Frame
	found := false
	for _, h := range rm.held {
		if h.Kind != wire.KindCopy {
			continue
		}
		if !found || h.Time > n.Time || (h.Time == n.Time && h.Seq > n.Seq) {
			n, found = h.Frame, true
		}
	}
	return n, found
}

func encode(f wire.Frame) []byte {
	b, err := f.Marshal()
	if err != nil {
		slog.Error("cannot encode a frame", "type", f.Type, "err", err)
	}
	return b
}

// devices are the person's devices, online or not.
func (r *relay) devices(ctx context.Context, owner string) (types.DevicesOutput, error) {
	rm, err := r.room(ctx, owner)
	if err != nil {
		return types.DevicesOutput{}, err
	}
	head, err := r.store.Head(ctx, owner)
	if err != nil {
		return types.DevicesOutput{}, err
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return types.DevicesOutput{Head: head, Devices: r.listDevices(rm)}, nil
}

// listDevices are the room's devices, the most recently seen first. The room
// is locked.
func (r *relay) listDevices(rm *room) []types.Device {
	counted := map[string]bool{}
	for _, d := range r.counted(rm) {
		counted[d.ID] = true
	}
	out := []types.Device{}
	for id, d := range rm.known {
		_, online := rm.conns[id]
		out = append(out, types.Device{
			ID: id, Name: d.Name, Online: online, LastSeen: d.LastSeen, Acked: d.Acked, Counted: counted[id],
		})
	}
	slices.SortFunc(out, func(a, b types.Device) int {
		return cmp.Or(b.LastSeen.Compare(a.LastSeen), strings.Compare(a.ID, b.ID))
	})
	return out
}

// queue is what the relay holds for the person, and which devices lack it.
func (r *relay) queue(ctx context.Context, owner string) (types.QueueOutput, error) {
	rm, err := r.room(ctx, owner)
	if err != nil {
		return types.QueueOutput{}, err
	}
	head, err := r.store.Head(ctx, owner)
	if err != nil {
		return types.QueueOutput{}, err
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()
	out := types.QueueOutput{Head: head, Devices: r.listDevices(rm), Queue: []types.QueueEntry{}}
	devices := r.counted(rm)
	for _, h := range rm.held {
		e := types.QueueEntry{
			Seq: h.Seq, Kind: string(h.Kind), Created: h.At().UTC(), Origin: h.Origin,
			Size: h.Size(), Waiting: []string{},
		}
		if len(h.Formats) > 0 {
			e.Type = types.MIME(h.Formats[0].MIME)
		}
		for _, d := range devices {
			if lacks(d, h.Seq) {
				e.Waiting = append(e.Waiting, d.ID)
			}
		}
		slices.Sort(e.Waiting)
		out.Queue = append(out.Queue, e)
	}
	return out, nil
}

// Errors taking a copy back can end in.
var (
	errNotHeld  = errors.New("the server holds no such copy")
	errArrived  = errors.New("a device has it, or is online to get it; delete it from the history instead")
	errNoSuchID = errors.New("no such device")
)

// takeBack voids a held copy no device has yet: its number stays, as a void,
// so the devices learn it is gone.
func (r *relay) takeBack(rm *room, seq uint64) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	i, ok := slices.BinarySearchFunc(rm.held, seq, func(h held, seq uint64) int { return cmp.Compare(h.Seq, seq) })
	if !ok || rm.held[i].Kind != wire.KindCopy {
		return errNotHeld
	}
	// A device online was sent it the moment it was numbered, and may have
	// it before it says so: only while every device is away can a copy be
	// taken back from all of them.
	if len(rm.conns) > 0 {
		return errArrived
	}
	for _, d := range rm.known {
		if !lacks(d, seq) {
			return errArrived
		}
	}
	rm.void(func(h *held) bool { return h.Seq == seq })
	b := encode(rm.held[i].Frame)
	for _, l := range rm.conns {
		l.send(b)
	}
	return nil
}

// forget stops the relay holding copies for the device id, until it connects
// again.
func (r *relay) forget(ctx context.Context, rm *room, id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	d, ok := rm.known[id]
	if !ok {
		return errNoSuchID
	}
	if err := r.store.ForgetDevice(ctx, rm.owner, id); err != nil {
		return err
	}
	d.Forgotten = true
	r.evict(rm)
	return nil
}

// copyHeld is the held copy numbered seq, if the relay holds it.
func (rm *room) copyHeld(seq uint64) (wire.Frame, bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	i, ok := slices.BinarySearchFunc(rm.held, seq, func(h held, seq uint64) int { return cmp.Compare(h.Seq, seq) })
	if !ok || rm.held[i].Kind != wire.KindCopy {
		return wire.Frame{}, false
	}
	return rm.held[i].Frame, true
}

// closeAll ends every device's connection, as the server stops: an upgraded
// connection is the server's no longer, and a shutdown would not end it. The
// devices reconnect, to whichever server is up.
func (r *relay) closeAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rm := range r.rooms {
		rm.mu.Lock()
		for _, l := range rm.conns {
			l.close()
		}
		rm.mu.Unlock()
	}
}
