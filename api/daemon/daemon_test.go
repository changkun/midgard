// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"net"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/types/proto"
	"changkun.de/x/midgard/internal/version"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// dial starts the daemon's gRPC service on an ephemeral port and returns a
// client connected to it. It exercises the generated protobuf code and the
// gRPC transport end to end.
func dial(t *testing.T) proto.MidgardClient {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}

	s := grpc.NewServer()
	proto.RegisterMidgardServer(s, NewDaemon())
	go s.Serve(l)
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient(l.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("cannot dial the daemon: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return proto.NewMidgardClient(conn)
}

func TestRPCPing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := dial(t).Ping(ctx, &proto.PingInput{})
	if err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	if out.GetGoVersion() != version.GoVersion {
		t.Fatalf("go version: got %q, want %q", out.GetGoVersion(), version.GoVersion)
	}
	if out.GetVersion() != version.GitVersion {
		t.Fatalf("version: got %q, want %q", out.GetVersion(), version.GitVersion)
	}
}

// TestRPCListDaemons pins the timeout path. ListDaemons asks the midgard
// server over a websocket that is not connected here, so it must report the
// deadline back to the caller instead of hanging or panicking.
func TestRPCListDaemons(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := dial(t).ListDaemons(ctx, &proto.ListDaemonsInput{})
	if err == nil {
		t.Fatal("ListDaemons succeeded without a server connection")
	}
}

func TestNewDaemonID(t *testing.T) {
	if id := NewDaemon().ID; id == "" {
		t.Fatal("daemon has an empty ID")
	}
}
