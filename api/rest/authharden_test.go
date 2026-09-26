// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func resetBlocklist(t *testing.T) {
	t.Helper()
	blocklist.Range(func(k, _ any) bool {
		blocklist.Delete(k)
		return true
	})
	blocklistSize.Store(0)
}

// login sends a clipboard read from remote, claiming to forward for xff.
func login(m *Midgard, remote, xff string, good bool) int {
	req := httptest.NewRequest(http.MethodGet, "/midgard/api/v1/clipboard", nil)
	req.RemoteAddr = remote
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	if good {
		req.Header.Set("Authorization", bearer())
	} else {
		req.Header.Set("Authorization", "Bearer not-a-token")
	}
	w := httptest.NewRecorder()
	m.routers().ServeHTTP(w, req)
	return w.Code
}

// TestForwardedForCannotEvadeBlock is the bypass: failures are counted per
// client address, and gin took that address from X-Forwarded-For whoever sent
// it, so a new header per guess meant never being blocked.
func TestForwardedForCannotEvadeBlock(t *testing.T) {
	resetBlocklist(t)
	m := testMidgard(t)
	const attacker = "203.0.113.9:4000" // public, so not a trusted proxy

	for i := range maxFailureAttempts + 1 {
		login(m, attacker, fmt.Sprintf("198.51.100.%d", i+1), false)
	}
	if code := login(m, attacker, "198.51.100.200", true); code != http.StatusForbidden {
		t.Fatalf("a blocked client with a fresh X-Forwarded-For got %d, want 403", code)
	}
}

// TestForwardedForFromProxy is the other half: behind a reverse proxy on a
// private network, clients are told apart by the header the proxy sets, so
// one client's failures do not block everyone behind the same proxy.
func TestForwardedForFromProxy(t *testing.T) {
	resetBlocklist(t)
	m := testMidgard(t)
	const proxy = "10.0.0.5:4000"

	for range maxFailureAttempts + 1 {
		login(m, proxy, "198.51.100.7", false)
	}
	if code := login(m, proxy, "198.51.100.7", true); code != http.StatusForbidden {
		t.Errorf("the failing client got %d, want 403", code)
	}
	if code := login(m, proxy, "198.51.100.8", true); code != http.StatusOK {
		t.Errorf("another client behind the same proxy got %d, want 200", code)
	}
}

// TestProfilesNeedLogin: the profiles were guarded by the Host header only,
// which the client chooses, and a heap profile holds the clipboard.
func TestProfilesNeedLogin(t *testing.T) {
	resetBlocklist(t)
	m := testMidgard(t)

	req := httptest.NewRequest(http.MethodGet, "/midgard/api/v1/debug/pprof/cmdline", nil)
	req.Host = "localhost"
	req.RemoteAddr = "127.0.0.1:4000"
	w := httptest.NewRecorder()
	m.routers().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("profile without a login got %d, want 401", w.Code)
	}

	if w := do(t, m, http.MethodGet, "/midgard/api/v1/debug/pprof/cmdline", "", true); w.Code != http.StatusOK {
		t.Fatalf("profile with a login got %d, want 200", w.Code)
	}
}

func TestSweep(t *testing.T) {
	resetBlocklist(t)
	now := time.Now().UTC()
	add := func(ip string, lastFail time.Time) {
		info := &blockinfo{failCount: 1}
		info.lastFail.Store(lastFail)
		info.blockTime.Store(10 * time.Second)
		blocklist.Store(ip, info)
		blocklistSize.Add(1)
	}
	add("192.0.2.1", now.Add(-time.Hour)) // long over
	add("192.0.2.2", now)                 // still blocking

	sweep(now)
	if _, ok := blocklist.Load("192.0.2.1"); ok {
		t.Error("an expired entry survived the sweep")
	}
	if _, ok := blocklist.Load("192.0.2.2"); !ok {
		t.Error("a current block was swept")
	}
	if n := blocklistSize.Load(); n != 1 {
		t.Errorf("blocklist size = %d after the sweep, want 1", n)
	}
}
