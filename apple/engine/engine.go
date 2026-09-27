// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/device"
	"changkun.de/x/midgard/internal/e2e"
	"changkun.de/x/midgard/internal/history"
	"changkun.de/x/midgard/internal/signin"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/wire"
)

// The engine as the app sees it: plain Go, which main.go gives a C face and
// the tests call directly, as a test cannot use cgo.

var (
	mu      sync.Mutex
	engine  *device.Engine
	stop    context.CancelFunc
	stopped chan struct{}
	release func()
)

// errNotStarted is returned when the app asks before starting the engine.
var errNotStarted = errors.New("the engine is not running")

// start opens the device's history and keeps it in sync, calling changed
// with each copy from another device that belongs on the clipboard. It
// fails when the server is not set, and while mg daemon, or another app,
// syncs this device.
func start(changed func(mime string, data []byte)) error {
	mu.Lock()
	defer mu.Unlock()
	if engine != nil {
		return nil
	}
	if err := config.Load(); err != nil {
		return errNoServer
	}
	rel, err := device.Lock()
	if err != nil {
		return err
	}
	fail := func(err error) error { rel(); return err }
	id, err := device.ID()
	if err != nil {
		return fail(err)
	}
	path, err := device.HistoryPath()
	if err != nil {
		return fail(err)
	}
	h, err := history.Open(path)
	if err != nil {
		return fail(err)
	}
	name, err := os.Hostname()
	if err != nil || name == "" {
		name = id
	}
	key, since, err := device.LoadKey()
	if err != nil {
		h.Close()
		return fail(fmt.Errorf("cannot read this Mac's key: %w", err))
	}
	e := &device.Engine{
		ID: id, Name: name, History: h, Keepalive: device.DefaultKeepalive, Dial: device.Dial,
		// the person's key (specs/redesign.md §11)
		Key: key, Since: since, LoadKey: device.LoadKey, SaveKey: device.SaveKey,
		Bridge: config.Get().PlainBridge,
		Changed: func(c history.Entry) {
			if mime, data, ok := firstFormat(c); ok && changed != nil {
				changed(mime, data)
			}
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); e.Run(ctx) }()
	engine, stop, stopped, release = e, cancel, done, rel
	return nil
}

// errNoServer is returned by start until the app has set the server.
var errNoServer = errors.New("no server is set")

// shutdown stops the sync and closes the history.
func shutdown() {
	mu.Lock()
	defer mu.Unlock()
	if engine == nil {
		return
	}
	stop()
	<-stopped
	engine.History.Close()
	release()
	engine = nil
}

func running() (*device.Engine, error) {
	mu.Lock()
	defer mu.Unlock()
	if engine == nil {
		return nil, errNotStarted
	}
	return engine, nil
}

// copyData records a copy made on the Mac, to send to the other devices.
func copyData(mime string, data []byte) error {
	e, err := running()
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > wire.MaxPayload {
		return errors.New("nothing to copy, or too much")
	}
	return e.Copy(context.Background(), mime, data)
}

func deleteCopy(seq uint64) error {
	e, err := running()
	if err != nil {
		return err
	}
	return e.Delete(context.Background(), seq)
}

func clearHistory() error {
	e, err := running()
	if err != nil {
		return err
	}
	return e.Clear(context.Background())
}

// entry is one copy as the app lists it.
type entry struct {
	Seq     uint64 `json:"seq"` // 0 while it waits to be numbered
	Time    int64  `json:"time"`
	Origin  string `json:"origin"`
	MIME    string `json:"mime"`
	Size    int    `json:"size"`
	Preview string `json:"preview,omitempty"` // the start of a text
	Waiting bool   `json:"waiting"`
	Ref     string `json:"ref,omitempty"` // its name in the outbox, while it waits
}

// previewLen is how much of a text the list shows.
const previewLen = 300

// historyJSON lists the newest n copies, newest first.
func historyJSON(n int) ([]byte, error) {
	e, err := running()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	list, err := e.History.Previews(ctx, n, previewLen+4) // a character is at most 4 bytes
	if err != nil {
		return nil, err
	}
	out := []entry{}
	for _, c := range list {
		en := entry{Seq: c.Seq, Time: c.Time.UnixMilli(), Origin: c.Origin, Size: c.Size(), Waiting: c.Waiting(), Ref: c.Ref}
		if len(c.Formats) > 0 {
			en.MIME = c.Formats[0].MIME
		}
		if c.Data != nil {
			en.Preview = preview(c.Data)
		}
		out = append(out, en)
	}
	return json.Marshal(out)
}

// preview is the start of a text, cut at a rune.
func preview(b []byte) string {
	if len(b) > previewLen {
		b = b[:previewLen]
		for len(b) > 0 && !utf8.Valid(b) {
			b = b[:len(b)-1]
		}
	}
	return string(b)
}

// get is the copy numbered seq, in its first format.
func get(seq uint64) (string, []byte, error) {
	e, err := running()
	if err != nil {
		return "", nil, err
	}
	c, err := e.History.Get(context.Background(), seq)
	if err != nil {
		return "", nil, err
	}
	mime, data, ok := firstFormat(c)
	if !ok {
		return "", nil, history.ErrNotFound
	}
	return mime, data, nil
}

// getWaiting is the copy waiting in the outbox as ref, in its first format.
func getWaiting(ref string) (string, []byte, error) {
	e, err := running()
	if err != nil {
		return "", nil, err
	}
	c, err := e.History.Waiting(context.Background(), ref)
	if err != nil {
		return "", nil, err
	}
	mime, data, ok := firstFormat(c)
	if !ok {
		return "", nil, history.ErrNotFound
	}
	return mime, data, nil
}

// takeBack takes the copy waiting as ref out of the outbox, before the
// server has it: it goes nowhere.
func takeBack(ref string) error {
	e, err := running()
	if err != nil {
		return err
	}
	return e.History.Forget(context.Background(), ref)
}

func firstFormat(c history.Entry) (string, []byte, bool) {
	if len(c.Formats) == 0 || c.Formats[0].Size > len(c.Data) {
		return "", nil, false
	}
	return c.Formats[0].MIME, c.Data[:c.Formats[0].Size], true
}

// status is what the app shows of the engine.
type status struct {
	Configured bool   `json:"configured"` // a server is set
	Server     string `json:"server,omitempty"`
	SignedIn   bool   `json:"signed_in"`
	Running    bool   `json:"running"`
	Online     bool   `json:"online"`
	Device     string `json:"device,omitempty"`
	Name       string `json:"name,omitempty"`
	// end-to-end encryption (specs/redesign.md §11): the Mac seals with its
	// person's key, or their key is one it lacks, and it must pair
	Sealing      bool `json:"sealing"`
	NeedsPairing bool `json:"needs_pairing"`
	// the server's allowlist does not have this Mac's person
	NotOnList bool `json:"not_on_list"`
	// a Shortcuts bridge: it hands iPhone Shortcuts copies in the clear
	Bridge bool `json:"bridge"`
}

func statusNow() status {
	var s status
	if config.Load() == nil {
		s.Configured, s.Server = true, config.ServerURL()
		s.SignedIn = signin.SignedIn()
	}
	if e, err := running(); err == nil {
		s.Running, s.Online, s.Device, s.Name = true, e.Online(), e.ID, e.Name
		s.Sealing = e.Sealing()
		s.NeedsPairing, _ = e.NeedsPairing()
		s.NotOnList = e.NotOnList()
		s.Bridge = config.Get().PlainBridge
	}
	return s
}

// pairShow leaves this Mac's key for another device, sealed under a new
// pairing code, and returns the code and the link a phone opens.
func pairShow() (code, link string, err error) {
	e, err := running()
	if err != nil {
		return "", "", err
	}
	k, since := e.PersonKey()
	if k == nil {
		return "", "", errors.New("this Mac does not have your key: pair it first")
	}
	c, err := device.ShowPairing(k, since)
	if err != nil {
		return "", "", err
	}
	return c.String(), config.ServerURL() + "/midgard/#" + c.Link(), nil
}

// setBridge switches the Mac's Shortcuts bridge on or off, and keeps it so.
func setBridge(on bool) error {
	e, err := running()
	if err != nil {
		return err
	}
	if err := config.SetPlainBridge(on); err != nil {
		return err
	}
	e.SetBridge(on)
	return nil
}

// pairJoin takes the person's key with a code another device showed, and
// syncs with it at once.
func pairJoin(given string) error {
	e, err := running()
	if err != nil {
		return err
	}
	c, err := e2e.ParseCode(given)
	if err != nil {
		return errors.New("that is not a pairing code: it is 26 letters and digits, as the other device shows it")
	}
	k, since, err := device.JoinPairing(c)
	if err != nil {
		return err
	}
	return e.SetKey(k, since)
}

// setup sets the server, for the first start.
func setup(domain string) error {
	_, err := config.Save(domain)
	return err
}

// login signs the Mac in, opening the approval link with open. It returns
// once approved, or after 15 minutes.
func login(open func(link string) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	return signin.LoginWith(ctx, open)
}

func logout() error { return signin.Logout() }

// share publishes data at a link, which it returns.
func share(mime string, data []byte) (string, error) {
	filename := ""
	if mime == string(types.MIMEImagePNG) {
		filename = "image.png"
	}
	sh, err := client.Share("", data, filename, 0)
	return sh.URL, err
}

func jsonOf(v any) ([]byte, error) { return json.Marshal(v) }
