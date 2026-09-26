// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
)

// Daemon is the midgard daemon that interact with midgard server.
type Daemon struct {
	ID      string
	writeCh chan *types.WebsocketMessage // writeCh is used for sending message along ws.
	url     string                       // the server's websocket, when not the configured one

	keepalive keepalive // how the connection to the server is kept alive
}

// NewDaemon creates a new midgard daemon
func NewDaemon() *Daemon {
	id, err := os.Hostname()
	if err != nil {
		id, err = utils.NewUUIDShort()
		if err != nil {
			panic(fmt.Errorf("failed to initialize daemon: %v", err))
		}
	}
	return &Daemon{
		ID:        id,
		writeCh:   make(chan *types.WebsocketMessage, 10),
		keepalive: defaultKeepalive,
	}
}

// Run runs the daemon: it keeps the connection to the server, and syncs the
// local clipboard over it.
func (m *Daemon) Run(ctx context.Context) (onStart, onStop func() error) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)

	run := func() {
		defer wg.Done()
		m.Serve(ctx)
	}

	onStart = func() error {
		wg.Add(1)
		go run()
		return nil
	}
	onStop = func() error {
		cancel()
		wg.Wait()
		return nil
	}
	return
}

// Serve keeps the daemon connected to the server and watches the local
// clipboard, until ctx is done. mg commands no longer reach it: they call the
// server directly (see internal/client).
func (m *Daemon) Serve(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		defer slog.Info("the websocket is terminated")
		m.stayConnected(ctx)
	})
	wg.Go(func() {
		defer slog.Info("the clipboard watcher is terminated")
		m.watchLocalClipboard(ctx)
	})
	wg.Wait()

	slog.Info("daemon is down, good bye")
}
