// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.design/x/midgard/pkg/config"
	"golang.design/x/midgard/pkg/types/proto"
	"google.golang.org/grpc"
)

// Midgard is the midgard daemon that interact with midgard server.
type Midgard struct {
	sync.Mutex
	id      string
	s       *grpc.Server
	ws      *websocket.Conn
	writeCh chan action
}

type action struct {
	Type    string
	Content string
}

// NewMidgard creates a new midgard daemon
func NewMidgard() *Midgard {
	return &Midgard{
		writeCh: make(chan action, 10),
	}
}

// Serve serves Midgard daemon:
// 1. maintaining midgard daemon rpc;
// 2. maintaining midgard daemon to server websocket.
func (m *Midgard) Serve() {
	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		q := make(chan os.Signal, 1)
		signal.Notify(q, os.Interrupt, os.Kill)
		sig := <-q
		log.Printf("%v", sig)
		log.Printf("shutting down midgard daemon ...")
		m.s.GracefulStop()
		cancel()
	}()
	go func() {
		// TODO: handle graceful connection close
		m.wsConnect()
		m.wsListen()
		m.wsClose()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.watchLocalClipboard(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.serveRPC()
	}()
	wg.Wait()

	log.Printf("daemon is down, good bye!")
}

const maxMessageSize = 10 << 20 // 10 MB

func (m *Midgard) serveRPC() {
	l, err := net.Listen("tcp", config.D().Addr)
	if err != nil {
		log.Fatalf("fail to initalize midgard daemon, err: %v", err)
	}

	m.s = grpc.NewServer(
		grpc.MaxMsgSize(maxMessageSize),
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
