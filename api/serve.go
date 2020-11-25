// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package api

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
	"golang.design/x/midgard/api/rest"
	"golang.design/x/midgard/api/rpc"
	"golang.design/x/midgard/cmd/version"
	"golang.design/x/midgard/config"
	"golang.design/x/midgard/pkg/types/proto"
	"google.golang.org/grpc"
)

// Midgard is the midgard server that serves all API endpoints.
type Midgard struct {
	s1 *http.Server
	s2 *grpc.Server
}

// NewMidgard creates a new midgard server
func NewMidgard() *Midgard {
	return &Midgard{}
}

// Serve serves Midgard servers, this contains two parts:
// 1. HTTP server: serves RESTful APIs
// 2. gRPC server: serves RPC endpoints for the Midgard CLI
func (m *Midgard) Serve() {
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

func (m *Midgard) serveHTTP() {
	m.s1 = &http.Server{Handler: m.routers(), Addr: config.S().HTTP}
	log.Printf("http server starting at http://%s", config.S().HTTP)
	err := m.s1.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Printf("close with error: %v", err)
	}
	return
}

func (m *Midgard) routers() (r *gin.Engine) {
	gin.SetMode(config.S().Mode)

	r = gin.Default()
	mg := r.Group("/midgard")
	mg.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, struct {
			Version   string `json:"version"`
			GoVersion string `json:"go_version"`
			BuildTime string `json:"build_time"`
		}{
			Version:   version.GitVersion,
			GoVersion: version.GoVersion,
			BuildTime: version.BuildTime,
		})
	})
	mg.Static(config.S().Store.Prefix, config.S().Store.Path)

	v1 := mg.Group("/api/v1", rest.BasicAuthWithAttemptsControl(rest.Credentials{
		config.S().Auth.User: config.S().Auth.Pass,
	}))
	{
		v1.GET("/clipboard", rest.GetFromUniversalClipboard)
		v1.POST("/clipboard", rest.PutToUniversalClipboard)
		v1.PUT("/allocate", rest.AllocateURL)
	}

	return
}

const maxMessageSize = 10 << 20 // 10 MB

func (m *Midgard) serveRPC() {
	l, err := net.Listen("tcp", config.S().RPC)
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
	log.Printf("rpc server starting at rpc://%s", config.S().RPC)
	if err := m.s2.Serve(l); err != nil {
		log.Fatalf("fail to serve rpc server, err: %v", err)
	}
}
