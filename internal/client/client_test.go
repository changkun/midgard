// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package client

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
		// testdata/config.yml gives the device an app token
		if got := r.Header.Get("Authorization"); got != "Bearer "+config.Get().Token {
			t.Errorf("Authorization = %q, want the device's token", got)
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

func TestHistory(t *testing.T) {
	server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /midgard/api/v1/history":
			json.NewEncoder(w).Encode(types.HistoryOutput{History: []types.HistoryEntry{{ID: 7, Device: "laptop", Type: types.MIMEPlainText, Size: 2}}})
		case "GET /midgard/api/v1/history/7":
			json.NewEncoder(w).Encode(types.ClipboardData{Type: types.MIMEImagePNG, Data: base64.StdEncoding.EncodeToString([]byte("png"))})
		case "DELETE /midgard/api/v1/history/7", "DELETE /midgard/api/v1/history":
			w.WriteHeader(http.StatusNoContent)
		case "GET /midgard/api/v1/history/8":
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"msg": "no such copy in your history"})
		default:
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"msg": "the disk is full"})
		}
	})

	h, err := History()
	if err != nil || len(h) != 1 || h[0].ID != 7 {
		t.Fatalf("History() = %+v, %v", h, err)
	}
	if typ, data, err := HistoryEntry(7); err != nil || typ != types.MIMEImagePNG || string(data) != "png" {
		t.Fatalf("HistoryEntry(7) = %v, %q, %v; want the decoded image", typ, data, err)
	}
	if _, _, err := HistoryEntry(8); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a missing copy: %v, want ErrNotFound", err)
	}
	if err := DeleteHistoryEntry(7); err != nil {
		t.Fatal(err)
	}
	if err := ClearHistory(); err != nil {
		t.Fatal(err)
	}
	if err := DeleteHistoryEntry(9); err == nil || !strings.Contains(err.Error(), "the disk is full") {
		t.Fatalf("a failure: %v, want the server's reason", err)
	}
}

func TestCopy(t *testing.T) {
	var got types.PutToUniversalClipboardInput
	server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/midgard/api/v1/clipboard" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&got)
		json.NewEncoder(w).Encode(types.PutToUniversalClipboardOutput{Message: "saved"})
	})
	if err := Copy(types.MIMEPlainText, []byte("a link")); err != nil || got.Type != types.MIMEPlainText || got.Data != "a link" {
		t.Fatalf("Copy(text) sent %+v, %v", got, err)
	}
	if err := Copy(types.MIMEImagePNG, []byte("png")); err != nil || got.Data != base64.StdEncoding.EncodeToString([]byte("png")) {
		t.Fatalf("Copy(image) sent %+v, %v; want it base64", got, err)
	}
}
