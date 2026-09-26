// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package client

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// server points the configuration at a fake midgard server for one test.
func server(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	saved := config.Get().Domain
	config.Get().Domain = srv.URL
	t.Cleanup(func() { config.Get().Domain = saved })
}

func TestAllocate(t *testing.T) {
	var got types.AllocateURLInput
	server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/midgard/api/v1/allocate" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != config.Authorization() {
			t.Errorf("the request carries no credentials")
		}
		json.NewDecoder(r.Body).Decode(&got)
		json.NewEncoder(w).Encode(types.AllocateURLOutput{URL: "/midgard/notes/a.png"})
	})

	url, err := Allocate("notes/a", []byte("png bytes"), "shot.png")
	if err != nil {
		t.Fatal(err)
	}
	if want := config.ServerURL() + "/midgard/notes/a.png"; url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
	data, _ := base64.StdEncoding.DecodeString(got.Data)
	if got.Source != types.SourceAttachment || string(data) != "png bytes" || got.URI != "notes/a.png" {
		t.Errorf("sent %+v, want the attachment at notes/a.png", got)
	}

	// no data: the server publishes the universal clipboard
	if _, err := Allocate("", nil, ""); err != nil {
		t.Fatal(err)
	}
	if got.Source != types.SourceUniversalClipboard || got.Data != "" || got.URI != "" {
		t.Errorf("sent %+v, want the universal clipboard at a random path", got)
	}
}

func TestAllocateReportsTheServersReason(t *testing.T) {
	server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.AllocateURLOutput{Message: "the requested uri already existed."})
	})
	if _, err := Allocate("taken", []byte("x"), ""); err == nil || err.Error() != "the requested uri already existed." {
		t.Fatalf("err = %v, want the server's reason", err)
	}
}

func TestDevices(t *testing.T) {
	server(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(types.DevicesOutput{Devices: []types.Device{{Index: 1, Name: "laptop"}}})
	})
	devices, err := Devices()
	if err != nil || len(devices) != 1 || devices[0].Name != "laptop" {
		t.Fatalf("Devices() = %+v, %v", devices, err)
	}
}
