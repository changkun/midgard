// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/version"
)

// do sends a request through the full router, including the auth
// middleware, and returns the recorded response.
func do(t *testing.T, m *Midgard, method, path, body string, auth bool) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.SetBasicAuth(config.S().Auth.User, config.S().Auth.Pass)
	}
	// the auth middleware blocks by client IP after repeated failures;
	// give every request its own so the subtests stay independent.
	req.RemoteAddr = "127.0.0.1:12345"

	w := httptest.NewRecorder()
	m.routers().ServeHTTP(w, req)
	return w
}

func TestPing(t *testing.T) {
	w := do(t, NewMidgard(), http.MethodGet, "/midgard/ping", "", false)
	if w.Code != http.StatusOK {
		t.Fatalf("ping: got %d, want %d", w.Code, http.StatusOK)
	}

	var out types.PingOutput
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("ping response is not valid json: %v", err)
	}
	if out.GoVersion != version.GoVersion {
		t.Fatalf("ping go version: got %q, want %q", out.GoVersion, version.GoVersion)
	}
}

func TestAuth(t *testing.T) {
	// blocklist is package state that outlives a single run; start from
	// a clean one so the failure counter cannot carry over.
	blocklist.Range(func(k, _ any) bool {
		blocklist.Delete(k)
		return true
	})

	m := NewMidgard()

	for i, tt := range []struct {
		name   string
		header string
		want   int
	}{
		{"no credentials", "", http.StatusUnauthorized},
		{"wrong credentials", "Basic " + base64.StdEncoding.EncodeToString(
			[]byte("nobody:wrong")), http.StatusUnauthorized},
		{"malformed header", "not-a-basic-header", http.StatusUnauthorized},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/midgard/api/v1/clipboard", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			// a fresh IP per case: the middleware blocks an IP after
			// maxFailureAttempts failures, which would mask the status
			// asserted here once the cases share a counter.
			req.RemoteAddr = fmt.Sprintf("10.0.0.%d:1234", i+1)

			w := httptest.NewRecorder()
			m.routers().ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Fatalf("got %d, want %d", w.Code, tt.want)
			}
		})
	}

	// the same endpoint with valid credentials must pass the middleware
	if w := do(t, m, http.MethodGet, "/midgard/api/v1/clipboard", "", true); w.Code != http.StatusOK {
		t.Fatalf("authenticated request: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUniversalClipboardText(t *testing.T) {
	// Universal.Write logs to ./data relative to the working
	// directory; keep that out of the source tree.
	t.Chdir(t.TempDir())

	m := NewMidgard()
	const want = "changkun.de/x/midgard"

	in, err := json.Marshal(types.PutToUniversalClipboardInput{
		ClipboardData: types.ClipboardData{Type: types.MIMEPlainText, Data: want},
	})
	if err != nil {
		t.Fatal(err)
	}

	w := do(t, m, http.MethodPost, "/midgard/api/v1/clipboard", string(in), true)
	if w.Code != http.StatusOK {
		t.Fatalf("put: got %d (%s), want %d", w.Code, w.Body, http.StatusOK)
	}

	w = do(t, m, http.MethodGet, "/midgard/api/v1/clipboard", "", true)
	if w.Code != http.StatusOK {
		t.Fatalf("get: got %d (%s), want %d", w.Code, w.Body, http.StatusOK)
	}

	var out types.GetFromUniversalClipboardOutput
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("get response is not valid json: %v", err)
	}
	if out.Type != types.MIMEPlainText || out.Data != want {
		t.Fatalf("round trip: got (%v, %q), want (%v, %q)",
			out.Type, out.Data, types.MIMEPlainText, want)
	}
}

func TestUniversalClipboardImage(t *testing.T) {
	// Universal.Write logs to ./data relative to the working
	// directory; keep that out of the source tree.
	t.Chdir(t.TempDir())

	m := NewMidgard()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var raw bytes.Buffer
	if err := png.Encode(&raw, img); err != nil {
		t.Fatal(err)
	}
	want := base64.StdEncoding.EncodeToString(raw.Bytes())

	in, err := json.Marshal(types.PutToUniversalClipboardInput{
		ClipboardData: types.ClipboardData{Type: types.MIMEImagePNG, Data: want},
	})
	if err != nil {
		t.Fatal(err)
	}

	if w := do(t, m, http.MethodPost, "/midgard/api/v1/clipboard", string(in), true); w.Code != http.StatusOK {
		t.Fatalf("put: got %d (%s), want %d", w.Code, w.Body, http.StatusOK)
	}

	w := do(t, m, http.MethodGet, "/midgard/api/v1/clipboard", "", true)
	var out types.GetFromUniversalClipboardOutput
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("get response is not valid json: %v", err)
	}
	// an image must survive as base64, not be mangled into a string
	if out.Type != types.MIMEImagePNG || out.Data != want {
		t.Fatalf("image round trip mismatch, got type %v, %d bytes of data, want type %v, %d bytes",
			out.Type, len(out.Data), types.MIMEImagePNG, len(want))
	}
}

func TestBindError(t *testing.T) {
	m := NewMidgard()

	for _, tt := range []struct {
		name, method, path, body string
	}{
		{"clipboard", http.MethodPost, "/midgard/api/v1/clipboard", "{not json"},
		{"allocate", http.MethodPut, "/midgard/api/v1/allocate", "{not json"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := do(t, m, tt.method, tt.path, tt.body, true)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("got %d (%s), want %d", w.Code, w.Body, http.StatusBadRequest)
			}
		})
	}
}
