// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package types_test

import (
	"os"
	"testing"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/testdata"
)

func TestMain(m *testing.M) {
	testdata.UseConfig()
	os.Exit(m.Run())
}

func TestEndpoints(t *testing.T) {
	saved := config.Get().Domain
	t.Cleanup(func() { config.Get().Domain = saved })

	for _, tt := range []struct{ domain, api, ws string }{
		{"example.com", "https://example.com/midgard/api/v1/clipboard", "wss://example.com/midgard/api/v1/ws"},
		{"http://mg.pi:8456", "http://mg.pi:8456/midgard/api/v1/clipboard", "ws://mg.pi:8456/midgard/api/v1/ws"},
		{"localhost:8080", "http://localhost:8080/midgard/api/v1/clipboard", "ws://localhost:8080/midgard/api/v1/ws"},
	} {
		config.Get().Domain = tt.domain
		if got := types.EndpointClipboard(); got != tt.api {
			t.Errorf("domain %q: EndpointClipboard() = %q, want %q", tt.domain, got, tt.api)
		}
		if got := types.EndpointSubscribe(); got != tt.ws {
			t.Errorf("domain %q: EndpointSubscribe() = %q, want %q", tt.domain, got, tt.ws)
		}
	}
}
