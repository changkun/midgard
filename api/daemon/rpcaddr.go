// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"changkun.de/x/midgard/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// rpcSocket is where the daemon listens for mg commands unless daemon.addr
// says otherwise: a Unix socket in a directory only this user can enter.
//
// It used to listen on localhost:9125, which every user and program on the
// machine can reach, and the daemon answers without asking who is calling —
// so anyone could have it publish the clipboard to a public link.
func rpcSocket() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR") // private to the user on Linux
	if dir == "" {
		var err error
		if dir, err = os.UserCacheDir(); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "midgard", "daemon.sock"), nil
}

// tcpAddr is the daemon.addr setting, if any.
func tcpAddr() string {
	if d := config.D(); d != nil {
		return d.Addr
	}
	return ""
}

// listenRPC opens the listener the daemon serves mg commands on, and
// describes it for the log.
func listenRPC() (net.Listener, string, error) {
	if addr := tcpAddr(); addr != "" {
		slog.Warn("the daemon listens on TCP, where any local user can reach it; "+
			"remove daemon.addr from config.yml to use a socket only you can", "addr", addr)
		l, err := net.Listen("tcp", addr)
		return l, "tcp://" + addr, err
	}

	path, err := rpcSocket()
	if err != nil {
		return nil, "", err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil { // in case it existed, looser
		return nil, "", err
	}
	// A socket left by a daemon that did not exit cleanly stops Listen; one
	// that still answers belongs to a daemon that is running.
	if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
		c.Close()
		return nil, "", fmt.Errorf("a daemon is already running at %s", path)
	}
	os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, "", err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		l.Close()
		return nil, "", err
	}
	return l, "unix://" + path, nil
}

// dialRPC connects an mg command to the daemon, wherever listenRPC put it.
func dialRPC() (*grpc.ClientConn, error) {
	creds := grpc.WithTransportCredentials(insecure.NewCredentials())
	if addr := tcpAddr(); addr != "" {
		return grpc.NewClient(addr, creds)
	}
	path, err := rpcSocket()
	if err != nil {
		return nil, err
	}
	// Dial the path directly rather than through a unix: target, which
	// gRPC parses as a URL and a Windows path is not.
	return grpc.NewClient("passthrough:///midgard-daemon", creds,
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", path)
		}))
}
