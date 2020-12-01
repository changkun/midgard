// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rest

import (
	"container/list"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.design/x/midgard/pkg/config"
)

// Midgard is the midgard server that serves all API endpoints.
type Midgard struct {
	s *http.Server

	mu    sync.Mutex
	users *list.List
}

// NewMidgard creates a new midgard server
func NewMidgard() *Midgard {
	return &Midgard{users: list.New()}
}

// Serve serves Midgard RESTful APIs.
func (m *Midgard) Serve() {
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		q := make(chan os.Signal, 1)
		signal.Notify(q, os.Interrupt, os.Kill)
		sig := <-q
		log.Printf("%v", sig)

		log.Printf("shutting down api service ...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := m.s.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
			log.Printf("failed to shudown api service: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		m.serveHTTP()
	}()
	wg.Wait()

	log.Printf("api server is down, good bye!")
}

func (m *Midgard) serveHTTP() {
	m.s = &http.Server{Handler: m.routers(), Addr: config.S().Addr}
	log.Printf("server starting at http://%s", config.S().Addr)
	err := m.s.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Printf("close with error: %v", err)
	}
	return
}

func (m *Midgard) routers() (r *gin.Engine) {
	gin.SetMode(config.S().Mode)

	r = gin.Default()
	r.LoadHTMLGlob(FixPath("./templates/*"))

	mg := r.Group("/midgard")
	mg.GET("/ping", m.PingPong)
	mg.GET("/news", m.News)
	mg.Static(config.S().Store.Prefix, config.S().Store.Path)

	v1 := mg.Group("/api/v1", BasicAuthWithAttemptsControl(Credentials{
		config.S().Auth.User: config.S().Auth.Pass,
	}))
	{
		v1.GET("/clipboard", m.GetFromUniversalClipboard)
		v1.POST("/clipboard", m.PutToUniversalClipboard)
		v1.GET("/ws", m.Subscribe)
		v1.PUT("/allocate", m.AllocateURL)
	}

	return
}

// FixPath fixes a relative path
func FixPath(p string) string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		log.Fatalf("cannot get runtime caller")
	}
	return path.Join(path.Dir(filename), p)
}
