// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/device"
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
	e := &device.Engine{
		ID: id, Name: name, History: h, Keepalive: device.DefaultKeepalive, Dial: device.Dial,
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
	list, err := e.History.List(ctx, n)
	if err != nil {
		return nil, err
	}
	out := []entry{}
	for _, c := range list {
		en := entry{Seq: c.Seq, Time: c.Time.UnixMilli(), Origin: c.Origin, Size: c.Size(), Waiting: c.Waiting()}
		if len(c.Formats) > 0 {
			en.MIME = c.Formats[0].MIME
		}
		if en.MIME == string(types.MIMEPlainText) && !c.Waiting() {
			if full, err := e.History.Get(ctx, c.Seq); err == nil {
				en.Preview = preview(full.Data)
			}
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
}

func statusNow() status {
	var s status
	if config.Load() == nil {
		s.Configured, s.Server = true, config.ServerURL()
		s.SignedIn = signin.SignedIn()
	}
	if e, err := running(); err == nil {
		s.Running, s.Online, s.Device, s.Name = true, e.Online(), e.ID, e.Name
	}
	return s
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
