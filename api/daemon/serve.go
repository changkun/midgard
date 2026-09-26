// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/device"
	"changkun.de/x/midgard/internal/history"
	"changkun.de/x/midgard/internal/types"
	"github.com/gorilla/websocket"
)

// Daemon is the device side of midgard without a desktop: it keeps the
// device's history in sync with the person's other devices, and the local
// clipboard with the newest copy (specs/redesign.md §3).
type Daemon struct {
	engine *device.Engine
	url    string // the server's websocket, when not the configured one

	mu      sync.Mutex
	written [sha256.Size]byte // what the daemon last put on the clipboard
}

// NewDaemon opens the device's history and sets up its sync. One midgard
// syncs a device at a time: it fails while the Mac app, or another daemon,
// does.
func NewDaemon() (*Daemon, error) {
	release, err := device.Lock()
	if err != nil {
		return nil, err
	}
	id, err := device.ID()
	if err != nil {
		release()
		return nil, fmt.Errorf("cannot name this device: %w", err)
	}
	path, err := device.HistoryPath()
	if err != nil {
		release()
		return nil, err
	}
	h, err := history.Open(path)
	if err != nil {
		release()
		return nil, fmt.Errorf("cannot open the history: %w", err)
	}
	name, err := os.Hostname()
	if err != nil || name == "" {
		name = id
	}
	m := &Daemon{}
	m.engine = &device.Engine{
		ID: id, Name: name, History: h, Keepalive: device.DefaultKeepalive,
		Dial: m.dial, Changed: m.changed,
	}
	return m, nil
}

// Run runs the daemon: it keeps the connection to the server, and syncs the
// local clipboard over it.
func (m *Daemon) Run(ctx context.Context) (onStart, onStop func() error) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	onStart = func() error {
		wg.Go(func() { m.Serve(ctx) })
		return nil
	}
	onStop = func() error {
		cancel()
		wg.Wait()
		return m.engine.History.Close()
	}
	return
}

// Serve keeps the device in sync and watches the local clipboard, until ctx
// is done.
func (m *Daemon) Serve(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		defer slog.Info("the sync is terminated")
		m.engine.Run(ctx)
	})
	wg.Go(func() {
		defer slog.Info("the clipboard watcher is terminated")
		m.watchLocalClipboard(ctx)
	})
	wg.Wait()
	slog.Info("daemon is down, good bye")
}

// dial connects to the server's websocket, signed in.
func (m *Daemon) dial(ctx context.Context) (*websocket.Conn, error) {
	if m.url != "" {
		return device.DialURL(ctx, m.url)
	}
	return device.Dial(ctx)
}

// changed puts a copy from another device on the local clipboard.
func (m *Daemon) changed(e history.Entry) {
	for i, f := range e.Formats {
		mime := types.MIME(f.MIME)
		if mime != types.MIMEPlainText && mime != types.MIMEImagePNG {
			continue
		}
		data := parts(e)[i]
		m.mu.Lock()
		m.written = sha256.Sum256(data)
		m.mu.Unlock()
		slog.Info("a copy arrived, putting it on the clipboard", "from", e.Origin, "mime", mime)
		clipboard.Local.Write(mime, data)
		return
	}
}

// ours reports whether data is what the daemon put on the clipboard itself,
// which the clipboard's watch reports as a change like any other.
func (m *Daemon) ours(data []byte) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.written == sha256.Sum256(data)
}

// parts splits a copy's bytes by its formats.
func parts(e history.Entry) [][]byte {
	out := make([][]byte, 0, len(e.Formats))
	rest := e.Data
	for _, f := range e.Formats {
		if f.Size > len(rest) {
			break
		}
		out = append(out, rest[:f.Size])
		rest = rest[f.Size:]
	}
	return out
}
