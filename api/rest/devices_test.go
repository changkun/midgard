// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"changkun.de/x/midgard/internal/types"
)

// TestDevices: mg daemon ls and mg status ask the server which daemons are
// connected, with a plain request instead of a round trip through a daemon.
func TestDevices(t *testing.T) {
	resetBlocklist(t)
	m := testMidgard(t)
	srv := httptest.NewServer(m.routers())
	t.Cleanup(srv.Close)
	subscribe(t, srv, "laptop")
	subscribe(t, srv, "desktop")

	w := do(t, m, http.MethodGet, "/midgard/api/v1/devices", "", true)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var out types.DevicesOutput
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, d := range out.Devices {
		names[d.Name] = true
	}
	if len(out.Devices) != 2 || !names["laptop"] || !names["desktop"] {
		t.Fatalf("devices = %+v, want laptop and desktop", out.Devices)
	}

	if w := do(t, m, http.MethodGet, "/midgard/api/v1/devices", "", false); w.Code != http.StatusUnauthorized {
		t.Fatalf("without a login: got %d, want 401", w.Code)
	}
}
