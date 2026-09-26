// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"container/list"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"changkun.de/x/midgard/internal/config"
)

// Midgard is the midgard server that serves all API endpoints.
type Midgard struct {
	s *http.Server

	mu    sync.Mutex
	users *list.List

	keepalive keepalive // how dead daemons are noticed
}

// NewMidgard creates a new midgard server
func NewMidgard() *Midgard {
	return &Midgard{users: list.New(), keepalive: defaultKeepalive}
}

// Serve serves Midgard RESTful APIs.
func (m *Midgard) Serve() {
	requirements()
	var wg sync.WaitGroup
	wg.Go(func() {
		// SIGTERM is how docker stop and systemd ask; only Ctrl-C was
		// caught before, so a stopped container was killed, not shut down.
		q := make(chan os.Signal, 1)
		signal.Notify(q, os.Interrupt, syscall.SIGTERM)
		sig := <-q
		slog.Info("received a signal", "signal", sig)

		slog.Info("shutting down the api service")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := m.s.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("cannot shut down the api service", "err", err)
		}
	})
	wg.Go(func() {
		m.serveHTTP()
	})
	wg.Wait()

	slog.Info("api server is down, good bye")
}

func (m *Midgard) serveHTTP() {
	addr := os.Getenv("MIDGARD_SERVER_ADDR")
	if len(addr) == 0 {
		addr = config.S().Addr
	}

	m.s = &http.Server{Handler: m.routers(), Addr: addr}
	slog.Info("api server is starting", "addr", "http://"+addr)
	err := m.s.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		slog.Error("api server closed with an error", "err", err)
	}
}

// examplePassword is the placeholder password in config.example.yml.
const examplePassword = "change-me"

// weakPassword reports whether p is a password nobody should serve with: none
// at all, or the published placeholder, which anyone can read.
func weakPassword(p string) bool { return p == "" || p == examplePassword }

// requirements checks what the server needs from the system it runs on. It
// runs when the server starts rather than at package init, because every mg
// command links this package and only the server needs these.
func requirements() {
	if weakPassword(config.S().Auth.Pass) {
		fatal("set server.auth.pass in config.yml: the server refuses to run " +
			"with an empty password or the one from config.example.yml")
	}
}

// fatal logs at the error level and exits, for a server that cannot start.
// log/slog has no Fatal.
func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
