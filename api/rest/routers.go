// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"net/http"
	"net/http/pprof"
	"path"
	"runtime"

	"changkun.de/x/midgard/internal/config"
	"github.com/gin-gonic/gin"
)

func (m *Midgard) routers() (r *gin.Engine) {
	gin.SetMode(config.S().Mode)

	r = gin.Default()
	// Believe X-Forwarded-For only from a proxy. gin believes it from
	// anyone by default, which let a client pick the address that failed
	// logins are counted against, and so never be blocked.
	proxies := config.S().TrustedProxies
	if len(proxies) == 0 {
		proxies = defaultTrustedProxies
	}
	if err := r.SetTrustedProxies(proxies); err != nil {
		fatal("invalid server.trusted_proxies", "err", err)
	}
	r.NoRoute(m.serveShare)

	mg := r.Group("/midgard")
	mg.GET("/ping", m.PingPong)
	mg.GET("/", m.WebPage)
	if m.web != nil {
		mg.GET("/.auth/login", m.web.login)
		mg.GET("/.auth/callback", m.web.callback)
		mg.GET("/.auth/logout", m.web.logout)
		mg.POST("/.auth/logout", m.web.logout)
	}

	v1auth := mg.Group("/api/v1", signIn(m.appTokens(), m.latere, m.web))
	{
		v1auth.GET("/clipboard", m.GetFromUniversalClipboard)
		v1auth.POST("/clipboard", m.PutToUniversalClipboard)
		v1auth.GET("/ws", notFromTheWeb, m.Subscribe)
		v1auth.GET("/devices", m.Devices)
		v1auth.POST("/shares", m.CreateShare)
		v1auth.GET("/shares", m.Shares)
		v1auth.DELETE("/shares/:slug", m.DeleteShare)
		v1auth.GET("/history", m.History)
		v1auth.DELETE("/history", m.ClearHistory)
		v1auth.GET("/history/:id", m.HistoryEntry)
		v1auth.DELETE("/history/:id", m.DeleteHistoryEntry)
		v1auth.GET("/tokens", byAPerson, m.Tokens)
		v1auth.POST("/tokens", byAPerson, m.IssueToken)
		v1auth.DELETE("/tokens/:name", byAPerson, m.RevokeToken)
	}

	// The profiles include a heap dump, which holds the clipboard and the
	// credentials, so they are behind the login like everything else, and
	// not for the web page's cookie.
	profile(v1auth.Group("", notFromTheWeb))
	return
}

// defaultTrustedProxies are loopback and the private networks.
var defaultTrustedProxies = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7",
}

// FixPath fixes a relative path
func FixPath(p string) string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		fatal("cannot get the runtime caller")
	}
	return path.Join(path.Dir(filename), p)
}

// profile the standard HandlerFuncs from the net/http/pprof package with
// the provided gin.Engine. prefixOptions is a optional. If not prefixOptions,
// the default path prefix is used, otherwise first prefixOptions will be path prefix.
//
// Basic Usage:
//
//   - use the pprof tool to look at the heap profile:
//     go tool pprof localhost:8080/midgard/api/v1/debug/pprof/heap
//   - look at a 30-second CPU profile:
//     go tool pprof localhost:8080/midgard/api/v1/debug/pprof/profile
//   - look at the goroutine blocking profile, after calling runtime.SetBlockProfileRate:
//     go tool pprof localhost:8080/midgard/api/v1/debug/pprof/block
//   - collect a 5-second execution trace:
//     go tool pprof localhost:8080/midgard/api/v1/debug/pprof/trace?seconds=5
func profile(r *gin.RouterGroup) {
	pprofHandler := func(h http.HandlerFunc) gin.HandlerFunc {
		return gin.WrapF(h)
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
