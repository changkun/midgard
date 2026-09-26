// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package config_test

import (
	"testing"

	"changkun.de/x/midgard/internal/config"
)

func TestServerURL(t *testing.T) {
	saved := config.Get().Domain
	t.Cleanup(func() { config.Get().Domain = saved })

	for domain, want := range map[string]string{
		"example.com":               "https://example.com",
		"example.com/":              "https://example.com",
		"localhost:8080":            "http://localhost:8080",
		"0.0.0.0:80":                "http://0.0.0.0:80",
		"http://mg.pi:8456":         "http://mg.pi:8456", // #27: http off localhost
		"https://example.com:8443/": "https://example.com:8443",
	} {
		config.Get().Domain = domain
		if got := config.ServerURL(); got != want {
			t.Errorf("domain %q: ServerURL() = %q, want %q", domain, got, want)
		}
	}
}
