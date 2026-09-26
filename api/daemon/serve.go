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
	"time"

	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/types/proto"
	"changkun.de/x/midgard/internal/utils"
	"google.golang.org/grpc"
)

// Daemon is the midgard daemon that interact with midgard server.
type Daemon struct {
	ID      string
	s       *grpc.Server
	readChs sync.Map                     // {string: chan *types.WebsocketMessage}
	writeCh chan *types.WebsocketMessage // writeCh is used for sending message along ws.
	url     string                       // the server's websocket, when not the configured one

	keepalive keepalive // how the connection to the server is kept alive

	proto.UnimplementedMidgardServer
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

// Run runs Daemon daemon:
// 1. maintaining midgard daemon rpc;
// 2. maintaining midgard daemon to server websocket.
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

// Serve serves Daemon daemon:
// 1. maintaining midgard daemon rpc;
// 2. maintaining midgard daemon to server websocket.
func (m *Daemon) Serve(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		defer slog.Info("the graceful shutdown assistant is terminated")
		<-ctx.Done()
		m.s.GracefulStop()
	})
	wg.Go(func() {
		defer slog.Info("the websocket is terminated")
		m.stayConnected(ctx)
	})
	wg.Go(func() {
		defer slog.Info("the clipboard watcher is terminated")
		m.watchLocalClipboard(ctx)
	})
	wg.Go(func() {
		defer slog.Info("the rpc server is terminated")
		m.serveRPC()
	})
	wg.Wait()

	slog.Info("daemon is down, good bye")
}

const maxMessageSize = 10 << 20 // 10 MB

func (m *Daemon) serveRPC() {
	l, addr, err := listenRPC()
	if err != nil {
		fatal("cannot initialize the midgard daemon", "err", err)
	}

	m.s = grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.MaxSendMsgSize(maxMessageSize),
		grpc.ConnectionTimeout(time.Minute*5),
	)
	proto.RegisterMidgardServer(m.s, m)
	slog.Info("daemon is running", "addr", addr)
	if err := m.s.Serve(l); err != nil {
		fatal("cannot serve the midgard daemon", "err", err)
	}
}

// fatal logs at the error level and exits. log/slog has no Fatal, and a
// daemon that cannot bind its rpc socket has nothing left to do.
func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
