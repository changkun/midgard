// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"changkun.de/x/midgard/internal/types"
)

func TestHistoryEndpoints(t *testing.T) {
	resetBlocklist(t)
	t.Chdir(t.TempDir())
	t.Setenv("AUTH_ALLOWED_PRINCIPALS", "alice@example.com, bob@example.com")
	m := testMidgard(t)
	srv := httptest.NewServer(m.routers())
	t.Cleanup(srv.Close)
	alice := person{"sub-alice", "alice@example.com"}
	bob := person{"sub-bob", "bob@example.com"}

	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG not really"))
	for _, body := range []string{
		`{"type":"text","data":"first"}`,
		`{"type":"image/png","data":"` + png + `"}`,
		`{"type":"text","data":"third"}`,
	} {
		if code, b := alice.request(t, srv, http.MethodPost, "/midgard/api/v1/clipboard", body); code != 200 {
			t.Fatalf("copying: %d %s", code, b)
		}
	}

	history := func(p person) []types.HistoryEntry {
		t.Helper()
		code, b := p.request(t, srv, http.MethodGet, "/midgard/api/v1/history", "")
		if code != 200 {
			t.Fatalf("listing %s's history: %d %s", p.sub, code, b)
		}
		var out types.HistoryOutput
		json.Unmarshal(b, &out)
		return out.History
	}
	h := history(alice)
	if len(h) != 3 || h[0].Type != types.MIMEPlainText || h[1].Type != types.MIMEImagePNG {
		t.Fatalf("alice's history = %+v, want three, newest first", h)
	}
	if got := history(bob); len(got) != 0 {
		t.Fatalf("bob's history lists %d of alice's copies", len(got))
	}

	// reading one back, the image as base64 like GET /clipboard
	code, b := alice.request(t, srv, http.MethodGet, fmt.Sprintf("/midgard/api/v1/history/%d", h[1].ID), "")
	var entry types.ClipboardData
	json.Unmarshal(b, &entry)
	if code != 200 || entry.Type != types.MIMEImagePNG || entry.Data != png {
		t.Fatalf("reading the image: %d %+v", code, entry)
	}

	// someone else's copy is not found, the same as one that never was
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		if code, _ := bob.request(t, srv, method, fmt.Sprintf("/midgard/api/v1/history/%d", h[0].ID), ""); code != http.StatusNotFound {
			t.Errorf("bob %s alice's copy: %d, want 404", method, code)
		}
	}
	if code, _ := alice.request(t, srv, http.MethodGet, "/midgard/api/v1/history/999999", ""); code != http.StatusNotFound {
		t.Errorf("a copy that never was: %d, want 404", code)
	}
	if code, _ := alice.request(t, srv, http.MethodGet, "/midgard/api/v1/history/latest", ""); code != http.StatusBadRequest {
		t.Errorf("a name that is not a number: %d, want 400", code)
	}

	// bob clearing his history leaves alice's
	bob.request(t, srv, http.MethodDelete, "/midgard/api/v1/history", "")
	if len(history(alice)) != 3 {
		t.Fatal("bob clearing his history touched alice's")
	}

	if code, _ := alice.request(t, srv, http.MethodDelete, fmt.Sprintf("/midgard/api/v1/history/%d", h[0].ID), ""); code != http.StatusNoContent {
		t.Fatalf("deleting: %d, want 204", code)
	}
	_, b = alice.request(t, srv, http.MethodGet, "/midgard/api/v1/clipboard", "")
	json.Unmarshal(b, &entry)
	if entry.Data != png {
		t.Errorf("after deleting the newest, the clipboard is %+v, want the image", entry)
	}
	alice.request(t, srv, http.MethodDelete, "/midgard/api/v1/history", "")
	if len(history(alice)) != 0 {
		t.Error("clearing left copies behind")
	}
}
