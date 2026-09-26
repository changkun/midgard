// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
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
	"changkun.de/x/midgard/internal/store"
)

// Midgard is the midgard server that serves all API endpoints.
type Midgard struct {
	s     *http.Server
	store *store.Store // nil until Serve opens it, or a test sets it

	mu    sync.Mutex
	relay *relay // passes copies between each person's devices; see rel

	keepalive keepalive   // how dead daemons are noticed
	latere    *latereAuth // sign-in through auth.latere.ai; nil when not set up
	web       *webAuth    // sign-in from a browser, for the web page; nil when not set up
}

// appTokens is what the login checks app tokens against: the store, when
// there is one.
func (m *Midgard) appTokens() appTokens {
	if m.store == nil {
		return nil
	}
	return m.store
}

// NewMidgard creates a new midgard server
func NewMidgard() *Midgard {
	return &Midgard{keepalive: defaultKeepalive, latere: newLatereAuth(), web: newWebAuth()}
}

// rel is the relay, on the server's store; made on first use, once the
// store is open.
func (m *Midgard) rel() *relay {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.relay == nil || m.relay.store != m.store {
		m.relay = newRelay(m.store)
	}
	return m.relay
}

// Serve serves Midgard RESTful APIs.
func (m *Midgard) Serve() {
	m.requirements()
	st, err := store.Open(config.DBPath)
	if err != nil {
		fatal("cannot open the database", "path", config.DBPath, "err", err)
	}
	defer st.Close()
	m.store = st
	var wg sync.WaitGroup
	wg.Go(func() {
		// SIGTERM is how docker stop and systemd ask; only Ctrl-C was
		// caught before, so a stopped container was killed, not shut down.
		q := make(chan os.Signal, 1)
		signal.Notify(q, os.Interrupt, syscall.SIGTERM)
		sig := <-q
		slog.Info("received a signal", "signal", sig)

		slog.Info("shutting down the api service")
		m.rel().closeAll()
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

// requirements checks what the server needs from the system it runs on. It
// runs when the server starts rather than at package init, because every mg
// command links this package and only the server needs these.
func (m *Midgard) requirements() {
	if m.latere == nil {
		fatal("set AUTH_ALLOWED_PRINCIPALS to who may use this server, by email " +
			"or principal id: with no one allowed, no one could sign in (see .env.template)")
	}
}

// fatal logs at the error level and exits, for a server that cannot start.
// log/slog has no Fatal.
func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
