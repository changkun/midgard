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
	"time"

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

func TestShare(t *testing.T) {
	var got types.ShareInput
	server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/midgard/api/v1/shares" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		// testdata/config.yml gives the device an app token
		if got := r.Header.Get("Authorization"); got != "Bearer "+config.Get().Token {
			t.Errorf("Authorization = %q, want the device's token", got)
		}
		got = types.ShareInput{}
		json.NewDecoder(r.Body).Decode(&got)
		json.NewEncoder(w).Encode(types.ShareInfo{Slug: "abc", URL: "/midgard/notes/a.png"})
	})

	sh, err := Share("notes/a", []byte("png bytes"), "shot.png", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if want := config.ServerURL() + "/midgard/notes/a.png"; sh.URL != want || sh.Slug != "abc" {
		t.Errorf("Share = %+v, want %q", sh, want)
	}
	data, _ := base64.StdEncoding.DecodeString(got.Data)
	if string(data) != "png bytes" || got.Type != types.MIMEImagePNG || got.Name != "notes/a.png" || got.ExpiresIn != 3600 {
		t.Errorf("sent %+v, want the png at notes/a.png for an hour", got)
	}

	// no file: the name is kept as it is
	if _, err := Share("notes/today.md", []byte("# hi"), "", 0); err != nil {
		t.Fatal(err)
	}
	if got.Name != "notes/today.md" || got.ExpiresIn != 0 {
		t.Errorf("sent %+v, want notes/today.md, never expiring", got)
	}

	// no data: the server shares the clipboard
	if _, err := Share("", nil, "", 0); err != nil {
		t.Fatal(err)
	}
	if got.Data != "" || got.Type != "" || got.Name != "" {
		t.Errorf("sent %+v, want the clipboard at a random link", got)
	}
}

func TestShareReportsTheServersReason(t *testing.T) {
	server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"msg": "taken is taken; pick another name"})
	})
	if _, err := Share("taken", []byte("x"), "", 0); err == nil || !strings.Contains(err.Error(), "taken is taken") {
		t.Fatalf("err = %v, want the server's reason", err)
	}
}

func TestShares(t *testing.T) {
	server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /midgard/api/v1/shares":
			json.NewEncoder(w).Encode(types.SharesOutput{Shares: []types.ShareInfo{{Slug: "abc", URL: "/midgard/s/abc"}}})
		case "DELETE /midgard/api/v1/shares/abc":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"msg": "you have no such share"})
		}
	})
	list, err := Shares()
	if err != nil || len(list) != 1 || list[0].URL != config.ServerURL()+"/midgard/s/abc" {
		t.Fatalf("Shares() = %+v, %v; want full links", list, err)
	}
	if err := DeleteShare("abc"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteShare("nosuch"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoking a missing share: %v, want ErrNotFound", err)
	}
}

func TestDevices(t *testing.T) {
	server(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(types.DevicesOutput{Devices: []types.Device{{ID: "a1", Name: "laptop", Online: true}}})
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
		switch r.Method + " " + r.URL.Path {
		case "POST /midgard/api/v1/clipboard":
			json.NewDecoder(r.Body).Decode(&got)
			json.NewEncoder(w).Encode(types.PutToUniversalClipboardOutput{Message: "copied", Seq: 57})
		case "GET /midgard/api/v1/clipboard":
			json.NewEncoder(w).Encode(types.ClipboardData{Type: types.MIMEImagePNG, Data: base64.StdEncoding.EncodeToString([]byte("png"))})
		default:
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
	})
	seq, err := Copy(types.MIMEPlainText, []byte("a link"))
	if err != nil || seq != 57 || got.Type != types.MIMEPlainText || got.Data != "a link" {
		t.Fatalf("Copy(text) = %d, %v; sent %+v", seq, err, got)
	}
	if _, err := Copy(types.MIMEImagePNG, []byte("png")); err != nil || got.Data != base64.StdEncoding.EncodeToString([]byte("png")) {
		t.Fatalf("Copy(image) sent %+v, %v; want it base64", got, err)
	}
	if typ, data, err := Clipboard(); err != nil || typ != types.MIMEImagePNG || string(data) != "png" {
		t.Fatalf("Clipboard() = %v, %q, %v; want the decoded image", typ, data, err)
	}
}

// TestNoDevice: a read the server cannot answer, with none of the person's
// devices online, is an error of its own, for mg to exit with its own code.
func TestNoDevice(t *testing.T) {
	server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"msg": "none of your devices is online"})
	})
	if _, _, err := Clipboard(); !errors.Is(err, ErrNoDevice) {
		t.Fatalf("Clipboard() with no device online: %v, want ErrNoDevice", err)
	}
	if _, err := History(); !errors.Is(err, ErrNoDevice) {
		t.Fatalf("History() with no device online: %v, want ErrNoDevice", err)
	}
}
