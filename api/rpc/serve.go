// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rpc

import (
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"golang.design/x/midgard/config"
	"golang.design/x/midgard/pkg/types/proto"
	"google.golang.org/grpc"
)

// Midgard is the midgard daemon that interact with midgard server.
type Midgard struct {
	s *grpc.Server
}

// NewMidgard creates a new midgard daemon
func NewMidgard() *Midgard {
	return &Midgard{}
}

// Serve serves Midgard servers, this contains two parts:
// 1. HTTP server: serves RESTful APIs
// 2. gRPC server: serves RPC endpoints for the Midgard CLI
func (m *Midgard) Serve() {
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		q := make(chan os.Signal, 1)
		signal.Notify(q, os.Interrupt, os.Kill)
		sig := <-q
		log.Printf("%v", sig)
		log.Printf("shutting down midgard daemon ...")
		m.s.GracefulStop()
	}()
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
		log.Fatalf("fail to init midgard daemon, err: %v", err)
	}

	m.s = grpc.NewServer(
		grpc.MaxMsgSize(maxMessageSize),
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.MaxSendMsgSize(maxMessageSize),
		grpc.ConnectionTimeout(time.Minute*5),
	)
	proto.RegisterMidgardServer(m.s, &Server{})
	log.Printf("daemon running at rpc://%s", config.D().Addr)
	if err := m.s.Serve(l); err != nil {
		log.Fatalf("fail to serve midgard daemon, err: %v", err)
	}
}
