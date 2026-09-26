// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"latere.ai/x/pkg/authkit/issuertest"
)

// The hard barrier (specs/redesign.md §5) is tested wherever data is: two
// people, shown to see nothing of each other's, through every endpoint. See
// TestClipboardsAreApart, TestSharesAreApart and the like.

// person is someone signed in, by the token they send.
type person struct{ sub, email string }

func (p person) header() string {
	return "Bearer " + testIssuer.Mint(issuertest.Claims{Sub: p.sub, Email: p.email})
}

func (p person) request(t *testing.T, srv *httptest.Server, method, path, body string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	req.Header.Set("Authorization", p.header())
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}
