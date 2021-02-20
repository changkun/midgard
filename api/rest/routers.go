// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"path"
	"runtime"
	"strings"

	"changkun.de/x/midgard/internal/config"
	"github.com/gin-gonic/gin"
)

func (m *Midgard) routers() (r *gin.Engine) {
	gin.SetMode(config.S().Mode)

	r = gin.Default()
	r.NoRoute(staticHandler(config.S().Store.Prefix, config.RepoPath))

	mg := r.Group("/midgard")
	mg.GET("/ping", m.PingPong)
	mg.GET("/code", m.Code)

	v1auth := mg.Group("/api/v1", BasicAuthWithAttemptsControl(Credentials{
		config.S().Auth.User: config.S().Auth.Pass,
	}))
	{
		v1auth.GET("/clipboard", m.GetFromUniversalClipboard)
		v1auth.POST("/clipboard", m.PutToUniversalClipboard)
		v1auth.GET("/ws", m.Subscribe)
		v1auth.PUT("/allocate", m.AllocateURL)
		v1auth.POST("/code2img", m.Code2img)
	}

	profile(mg.Group("/api/v1"))
	return
}

func staticHandler(prefix, root string) gin.HandlerFunc {
	fs := gin.Dir(root, false)
	fileServer := http.StripPrefix(prefix, http.FileServer(fs))

	return func(c *gin.Context) {
		file := strings.TrimPrefix(c.Request.URL.String(), prefix)
		// Check if file exists and/or if we have permission to access it
		f, err := fs.Open(file)
		if err != nil {
			c.Writer.WriteHeader(http.StatusNotFound)
			return
		}
		f.Close()
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}

// FixPath fixes a relative path
func FixPath(p string) string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		log.Fatalf("cannot get runtime caller")
	}
	return path.Join(path.Dir(filename), p)
}

// profile the standard HandlerFuncs from the net/http/pprof package with
// the provided gin.Engine. prefixOptions is a optional. If not prefixOptions,
// the default path prefix is used, otherwise first prefixOptions will be path prefix.
//
// Basic Usage:
//
// - use the pprof tool to look at the heap profile:
//   go tool pprof localhost:8080/midgard/api/v1/debug/pprof/heap
// - look at a 30-second CPU profile:
//   go tool pprof localhost:8080/midgard/api/v1/debug/pprof/profile
// - look at the goroutine blocking profile, after calling runtime.SetBlockProfileRate:
//   go tool pprof localhost:8080/midgard/api/v1/debug/pprof/block
// - collect a 5-second execution trace:
//   go tool pprof localhost:8080/midgard/api/v1/debug/pprof/trace?seconds=5
//
func profile(r *gin.RouterGroup) {
	pprofHandler := func(h http.HandlerFunc) gin.HandlerFunc {
		handler := http.HandlerFunc(h)
		return func(c *gin.Context) {

			fmt.Println(c.Request.Host)
			if !strings.Contains(c.Request.Host, "localhost") {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			handler.ServeHTTP(c.Writer, c.Request)
		}
	}
	rr := r.Group("/debug/pprof")
	{
		rr.GET("/", pprofHandler(pprof.Index))
		rr.GET("/cmdline", pprofHandler(pprof.Cmdline))
		rr.GET("/profile", pprofHandler(pprof.Profile))
		rr.POST("/symbol", pprofHandler(pprof.Symbol))
		rr.GET("/symbol", pprofHandler(pprof.Symbol))
		rr.GET("/trace", pprofHandler(pprof.Trace))
		rr.GET("/allocs", pprofHandler(pprof.Handler("allocs").ServeHTTP))
		rr.GET("/block", pprofHandler(pprof.Handler("block").ServeHTTP))
		rr.GET("/goroutine", pprofHandler(pprof.Handler("goroutine").ServeHTTP))
		rr.GET("/heap", pprofHandler(pprof.Handler("heap").ServeHTTP))
		rr.GET("/mutex", pprofHandler(pprof.Handler("mutex").ServeHTTP))
		rr.GET("/threadcreate", pprofHandler(pprof.Handler("threadcreate").ServeHTTP))
	}
}
