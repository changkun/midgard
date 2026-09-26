// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types/proto"
	"google.golang.org/grpc"
)

// runtimeDir points the socket at a fresh directory, short enough for a
// socket path on every system.
func runtimeDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mg")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", dir)
	return dir
}

func serveOn(t *testing.T, l net.Listener) {
	t.Helper()
	s := grpc.NewServer()
	proto.RegisterMidgardServer(s, NewDaemon())
	go s.Serve(l)
	t.Cleanup(s.Stop)
}

// TestRPCOverSocket: mg commands reach the daemon over a socket only its user
// can open, instead of a TCP port every local user could.
func TestRPCOverSocket(t *testing.T) {
	dir := runtimeDir(t)
	l, addr, err := listenRPC()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(addr, "unix://") {
		t.Fatalf("listening on %s, want a unix socket", addr)
	}
	serveOn(t, l)

	if runtime.GOOS != "windows" {
		path, _ := rpcSocket()
		for p, want := range map[string]os.FileMode{filepath.Join(dir, "midgard"): 0o700, path: 0o600} {
			fi, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := fi.Mode().Perm(); got != want {
				t.Errorf("%s mode = %v, want %v", p, got, want)
			}
		}
	}

	conn, err := dialRPC()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := proto.NewMidgardClient(conn).Ping(ctx, &proto.PingInput{}); err != nil {
		t.Fatalf("ping over the socket: %v", err)
	}

	// A second daemon must not take the socket from the running one.
	if l2, _, err := listenRPC(); err == nil {
		l2.Close()
		t.Fatal("a second daemon took over the socket")
	}
}

// TestStaleSocketIsReplaced: a daemon that did not exit cleanly leaves its
// socket behind, which must not stop the next one from starting.
func TestStaleSocketIsReplaced(t *testing.T) {
	runtimeDir(t)
	path, _ := rpcSocket()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close() // the file stays, with nobody listening

	l, _, err = listenRPC()
	if err != nil {
		t.Fatalf("a stale socket stopped the daemon: %v", err)
	}
	l.Close()
}

// TestTCPAddrIsHonoured keeps daemon.addr working for configurations that
// set it.
func TestTCPAddrIsHonoured(t *testing.T) {
	saved := config.D().Addr
	t.Cleanup(func() { config.D().Addr = saved })
	config.D().Addr = "127.0.0.1:0"

	l, addr, err := listenRPC()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if !strings.HasPrefix(addr, "tcp://") || l.Addr().Network() != "tcp" {
		t.Fatalf("listening on %s (%s), want TCP", addr, l.Addr().Network())
	}
}
