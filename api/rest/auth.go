// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"changkun.de/x/midgard/internal/store"
	"github.com/gin-gonic/gin"
)

// blocklist holds the ip that should be blocked for further requests.
//
// It lives in memory only, so a restart forgets every block. It is swept
// of addresses whose block has run out once it holds sweepAbove of them,
// so failures from many addresses cannot grow it without bound.
//
// FIXME: persist it, so that restarting the server does not lift blocks.
var (
	blocklist     sync.Map // map[string]*blockinfo{}
	blocklistSize atomic.Int64
)

// sweepAbove is how many addresses the blocklist holds before it is swept.
const sweepAbove = 1024

// sweep forgets every address whose last failure, and the block it earned,
// are both over.
func sweep(now time.Time) {
	blocklist.Range(func(k, v any) bool {
		info := v.(*blockinfo)
		last := info.lastFail.Load().(time.Time)
		bloc := info.blockTime.Load().(time.Duration)
		if now.After(last.Add(bloc)) {
			if _, loaded := blocklist.LoadAndDelete(k); loaded {
				blocklistSize.Add(-1)
			}
		}
		return true
	})
}

type blockinfo struct {
	failCount int64
	lastFail  atomic.Value // time.Time
	blockTime atomic.Value // time.Duration
}

const maxFailureAttempts = 5

// appTokens finds whom an app token acts for; *store.Store is one.
type appTokens interface {
	CheckAppToken(ctx context.Context, tok string) (store.TokenHolder, bool)
}

// Keys the middleware sets on an authenticated request. The owner is whose
// data the request may reach, and the only place handlers take it from; the
// email is theirs, when known; via is how they signed in.
const (
	ctxOwner  = "midgard_owner"
	ctxEmail  = "midgard_email"
	ctxDevice = "midgard_device"
	ctxVia    = "midgard_via"
)

// The ways a request is signed in.
const (
	viaLatere   = "latere"    // a token auth.latere.ai minted for midgard
	viaAppToken = "app-token" // an app token
	viaSession  = "session"   // the web page's session cookie
)

// signIn admits a request that carries, as "Bearer <token>", a token
// auth.latere.ai minted for midgard or an app token, or else the web page's
// session cookie, belonging to someone on the allowlist, and records whose
// data it may reach. Anything else counts against the address, and enough
// failures block it for a while. latere may be nil, and then no one is
// admitted; tokens and web may be nil.
//
// A cookie is sent with every request to the site, whoever made the page
// that sends it, so a request the cookie signs in may change nothing unless
// it also carries the page's CSRF token.
//
// There is no password any more: sign-in names a person, and an app token
// names its owner, so an app token is only as good as its owner's place on
// the allowlist, and removing someone removes their Shortcuts too.
func signIn(tokens appTokens, latere *latereAuth, web *webAuth) gin.HandlerFunc {
	return func(c *gin.Context) {
		// check if the IP failure attempts are too much
		// if so, direct abort the request without checking credentials
		ip := c.ClientIP()
		if i, ok := blocklist.Load(ip); ok {
			info := i.(*blockinfo)
			count := atomic.LoadInt64(&info.failCount)
			if count > maxFailureAttempts {
				// if the ip is under block, then directly abort
				last := info.lastFail.Load().(time.Time)
				bloc := info.blockTime.Load().(time.Duration)

				if time.Now().UTC().Sub(last.Add(bloc)) < 0 {
					slog.Warn("blocking an ip",
						"ip", ip, "duration", bloc, "until", last.Add(bloc))
					c.AbortWithStatus(http.StatusForbidden)
					return
				}

				// clear the failcount, but increase the next block time
				atomic.StoreInt64(&info.failCount, 0)
				info.blockTime.Store(bloc * 2)
			}

		}

		var owner, email, device, via string
		found := false
		auth := c.Request.Header.Get("Authorization")
		if bearer, ok := strings.CutPrefix(auth, "Bearer "); ok {
			switch {
			case strings.HasPrefix(bearer, store.TokenPrefix):
				if tokens == nil {
					break
				}
				if h, ok := tokens.CheckAppToken(c.Request.Context(), bearer); ok && latere.permits(h.Owner, h.Email) {
					owner, email, device, via, found = h.Owner, h.Email, h.Name, viaAppToken, true
				}
			default:
				if id, ok := latere.identify(c.Request); ok {
					owner, email, via, found = id.Sub, id.Email, viaLatere, true
				}
			}
		} else if auth == "" {
			if sub, mail, ok := web.session(c.Writer, c.Request); ok && latere.permits(sub, mail) {
				owner, email, device, via, found = sub, mail, "web", viaSession, true
			}
		}
		if !found {
			if i, ok := blocklist.Load(ip); !ok {
				info := &blockinfo{
					failCount: 1,
				}
				info.lastFail.Store(time.Now().UTC())
				info.blockTime.Store(time.Second * 10)

				if _, loaded := blocklist.LoadOrStore(ip, info); !loaded {
					if blocklistSize.Add(1) > sweepAbove {
						sweep(time.Now().UTC())
					}
				}
			} else {
				info := i.(*blockinfo)
				atomic.AddInt64(&info.failCount, 1)
				info.lastFail.Store(time.Now().UTC())
			}

			c.Header("WWW-Authenticate", `Bearer realm="midgard"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if via == viaSession && !safeMethod(c.Request.Method) && !web.checkCSRF(c.Request) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "the request lacks the page's CSRF token; reload the page"})
			return
		}
		c.Set(ctxOwner, owner)
		c.Set(ctxEmail, email)
		c.Set(ctxDevice, device)
		c.Set(ctxVia, via)
	}
}

// safeMethod reports whether a request with method changes nothing.
func safeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// notFromTheWeb refuses a request the web page's cookie signed in. The
// websocket and the profiles are for daemons and their operator; the page
// needs neither, and a page elsewhere on the site must not reach them with
// a visitor's cookie.
func notFromTheWeb(c *gin.Context) {
	if c.GetString(ctxVia) == viaSession {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": "sign in with a token, not the web page's session"})
	}
}

// byAPerson refuses a request an app token signed in: only a person, signed
// in, may issue and revoke app tokens, or one leaked token could mint more.
func byAPerson(c *gin.Context) {
	if c.GetString(ctxVia) == viaAppToken {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "an app token cannot manage app tokens; sign in"})
	}
}
