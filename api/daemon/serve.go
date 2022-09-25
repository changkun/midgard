// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/office"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/types/proto"
	"changkun.de/x/midgard/internal/utils"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

// Daemon is the midgard daemon that interact with midgard server.
type Daemon struct {
	sync.Mutex

	ID          string
	status      office.Status
	forceUpdate chan struct{}
	s           *grpc.Server
	ws          *websocket.Conn
	readChs     sync.Map                     // {string: chan *types.WebsocketMessage}
	writeCh     chan *types.WebsocketMessage // writeCh is used for sending message along ws.

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
		ID:          id,
		forceUpdate: make(chan struct{}, 1),
		writeCh:     make(chan *types.WebsocketMessage, 10),
	}
}

// Run runs Daemon daemon:
// 1. maintaining midgard daemon rpc;
// 2. maintaining midgard daemon to server websocket.
func (m *Daemon) Run(ctx context.Context) (onStart, onStop func() error) {
	wg := sync.WaitGroup{}
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
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Println("graceful shutdown assistant is terminated.")
		<-ctx.Done()
		m.s.GracefulStop()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Println("websocket is terminated.")
		m.wsConnect()
		m.handleIO(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Println("clipboard watcher is terminated.")
		m.watchLocalClipboard(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Println("office watcher is terminated.")
		m.watchOfficeStatus(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Println("rpc server is terminated.")
		m.serveRPC()
	}()
	wg.Wait()

	log.Printf("daemon is down, good bye!")
}

const maxMessageSize = 10 << 20 // 10 MB

func (m *Daemon) serveRPC() {
	l, err := net.Listen("tcp", config.D().Addr)
	if err != nil {
		log.Fatalf("fail to initialize midgard daemon, err: %v", err)
	}

	m.s = grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.MaxSendMsgSize(maxMessageSize),
		grpc.ConnectionTimeout(time.Minute*5),
	)
	proto.RegisterMidgardServer(m.s, m)
	log.Printf("daemon running at rpc://%s", config.D().Addr)
	if err := m.s.Serve(l); err != nil {
		log.Fatalf("fail to serve midgard daemon, err: %v", err)
	}
}
