// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"changkun.de/x/midgard/internal/token"
	"changkun.de/x/midgard/internal/utils"
	"github.com/gin-gonic/gin"
)

// BasicAuth with attempt control

type authPair struct {
	value string
	user  string
}

type authPairs []authPair

func (a authPairs) searchCredential(authValue string) (string, bool) {
	if authValue == "" {
		return "", false
	}
	for _, pair := range a {
		// Compare in constant time, so how long a wrong guess takes
		// says nothing about how much of it was right.
		if subtle.ConstantTimeCompare([]byte(pair.value), []byte(authValue)) == 1 {
			return pair.user, true
		}
	}
	return "", false
}

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

// Credentials is the basic auth authentication credentials
type Credentials map[string]string

// BasicAuthWithAttemptsControl offers basic auth with maximum failure control.
// It also accepts a device token from tokens, sent as "Bearer <token>"; a
// failed token counts against the address like a wrong password.
func BasicAuthWithAttemptsControl(creds Credentials, tokens *token.Store) gin.HandlerFunc {
	realm := "Basic realm=" + strconv.Quote("Authorization Required")
	pairs := processCreds(creds)
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

		// Search user in the slice of allowed credentials, or the device
		// tokens for a bearer.
		header := c.Request.Header.Get("Authorization")
		user, found := pairs.searchCredential(header)
		if bearer, ok := strings.CutPrefix(header, "Bearer "); ok && tokens != nil {
			if device, ok := tokens.Check(bearer); ok {
				user, found = "device:"+device, true
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

			// Credentials doesn't match, we return 401 and abort handlers chain.
			c.Header("WWW-Authenticate", realm)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// The user credentials was found, set user's id to key
		// in this context.
		c.Set("midgard_user", user)
	}
}

func processCreds(creds Credentials) authPairs {
	if len(creds) <= 0 {
		panic("empty list of authorized credentials")
	}
	pairs := make(authPairs, 0, len(creds))
	for user, password := range creds {
		if user == "" {
			panic("user can not be empty")
		}
		base := user + ":" + password
		value := "Basic " + base64.StdEncoding.EncodeToString(utils.StringToBytes(base))
		pairs = append(pairs, authPair{
			value: value,
			user:  user,
		})
	}
	return pairs
}
