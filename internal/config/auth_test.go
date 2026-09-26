// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package config_test

import (
	"encoding/base64"
	"testing"

	"changkun.de/x/midgard/internal/config"
)

func TestAuthorization(t *testing.T) {
	saved := config.Get().Token
	t.Cleanup(func() { config.Get().Token = saved })

	config.Get().Token = ""
	creds := config.S().Auth.User + ":" + config.S().Auth.Pass
	if got, want := config.Authorization(), "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)); got != want {
		t.Errorf("without a token: %q, want %q", got, want)
	}

	config.Get().Token = "mgt_example"
	if got := config.Authorization(); got != "Bearer mgt_example" {
		t.Errorf("with a token: %q, want the bearer", got)
	}
}
