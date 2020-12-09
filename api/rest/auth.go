// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/base64"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

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
		if pair.value == authValue {
			return pair.user, true
		}
	}
	return "", false
}

// blocklist holds the ip that should be blocked for further requests.
//
// FIXME: this map may keep grow without releasing memory because of
// continuously attempts. we also do not persist this type of block info
// to the disk, which means if we reboot the service then all the blocker
// are gone and they can attack the server again.
var blocklist sync.Map // map[string]*blockinfo{}

type blockinfo struct {
	failCount int64
	lastFail  atomic.Value // time.Time
	blockTime atomic.Value // time.Duration
}

const (
	maxFailureAttempts = 5
)

// Credentials is the basic auth authentication credentials
type Credentials map[string]string

// BasicAuthWithAttemptsControl offers basic auth with maximum failure control.
func BasicAuthWithAttemptsControl(creds Credentials) gin.HandlerFunc {
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
					log.Printf("block ip %v, block time: %v, release until: %v\n",
						ip, bloc, last.Add(bloc))
					c.AbortWithStatus(http.StatusForbidden)
					return
				}

				// clear the failcount, but increase the next block time
				atomic.StoreInt64(&info.failCount, 0)
				info.blockTime.Store(bloc * 2)
			}

		}

		// Search user in the slice of allowed credentials
		user, found := pairs.searchCredential(c.Request.Header.Get("Authorization"))
		if !found {
			if i, ok := blocklist.Load(ip); !ok {
				info := &blockinfo{
					failCount: 1,
				}
				info.lastFail.Store(time.Now().UTC())
				info.blockTime.Store(time.Second * 10)

				blocklist.Store(ip, info)
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
