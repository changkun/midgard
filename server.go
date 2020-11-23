// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.design/x/midgard/api"
	"golang.design/x/midgard/api/proto"
	"golang.design/x/midgard/api/rpc"
	"golang.design/x/midgard/config"
	"google.golang.org/grpc"
)

// midgard is the midgard server that serves all routers
type midgard struct {
	s1 *http.Server
	s2 *grpc.Server
}

// New creates a new Midgard server
func newServer() *midgard {
	return &midgard{}
}

func (m *midgard) serve() {
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		q := make(chan os.Signal, 1)
		signal.Notify(q, os.Interrupt, os.Kill)
		sig := <-q
		log.Printf("%v", sig)

		log.Printf("shutting down http service ...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := m.s1.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
			log.Printf("failed to shudown: %v", err)
		}

		log.Printf("shutting down rpc service ...")
		m.s2.GracefulStop()
	}()
	go func() {
		defer wg.Done()
		m.serveHTTP()
	}()
	go func() {
		defer wg.Done()
		m.serveRPC()
	}()
	wg.Wait()

	log.Printf("server is down, good bye!")
}

func (m *midgard) serveHTTP() {
	m.s1 = &http.Server{Handler: m.routers(), Addr: config.Get().Addr.HTTP}
	log.Printf("http server starting at http://%s", config.Get().Addr.HTTP)
	err := m.s1.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Printf("close with error: %v", err)
	}
	return
}

func (m *midgard) routers() (r *gin.Engine) {
	gin.SetMode(config.Get().Mode)

	r = gin.Default()
	midgard := r.Group("/midgard")
	midgard.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, struct {
			Version   string `json:"version"`
			GoVersion string `json:"go_version"`
			BuildTime string `json:"build_time"`
		}{
			Version:   config.Version,
			GoVersion: config.GoVersion,
			BuildTime: config.BuildTime,
		})
	})
	midgard.Static(config.Get().Store.Prefix, config.Get().Store.Path)

	v1 := midgard.Group("/api/v1", gin.BasicAuth(gin.Accounts{
		config.Get().Auth.User: config.Get().Auth.Pass,
	}))
	{
		v1.GET("/clipboard", api.GetFromUniversalClipboard)
		v1.POST("/clipboard", api.PutToUniversalClipboard)
		v1.PUT("/generate", api.URIGenerator)
	}

	return
}

const maxMessageSize = 10 << 20 // 10 MB

func (m *midgard) serveRPC() {
	l, err := net.Listen("tcp", config.Get().Addr.RPC)
	if err != nil {
		log.Fatalf("fail to init rpc server, err: %v", err)
	}

	m.s2 = grpc.NewServer(
		grpc.MaxMsgSize(maxMessageSize),
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.MaxSendMsgSize(maxMessageSize),
		grpc.ConnectionTimeout(time.Minute*5),
	)
	proto.RegisterMidgardServer(m.s2, &rpc.Server{})
	log.Printf("rpc server starting at rpc://%s", config.Get().Addr.RPC)
	if err := m.s2.Serve(l); err != nil {
		log.Fatalf("fail to serve rpc server, err: %v", err)
	}
}
