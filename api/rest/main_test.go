// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"changkun.de/x/midgard/testdata"
	"latere.ai/x/pkg/authkit/issuertest"
)

// testIssuer stands in for auth.latere.ai in every test of this package, and
// testUser is on its allowlist.
var testIssuer *issuertest.Server

const (
	testUser  = "sub-test"
	testEmail = "test@example.com"
)

// bearer is the Authorization header of a request testUser signed in for.
func bearer() string {
	return "Bearer " + testIssuer.Mint(issuertest.Claims{Sub: testUser, Email: testEmail})
}

func TestMain(m *testing.M) {
	testdata.UseConfig()
	// NewHandler serves nothing by itself (New, which does, wants a *T), so
	// serve it here, naming the issuer by the address before it starts.
	var handler http.Handler
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}))
	testIssuer = issuertest.NewHandler(
		issuertest.WithIssuer("http://"+srv.Listener.Addr().String()),
		issuertest.WithDefaultAudience(audience),
	)
	handler = testIssuer.Handler()
	srv.Start()

	os.Setenv("AUTH_URL", testIssuer.URL())
	os.Setenv("AUTH_JWKS_URL", testIssuer.JWKSURL())
	os.Setenv("AUTH_ALLOWED_PRINCIPALS", testEmail)
	code := m.Run()
	srv.Close()
	os.Exit(code)
}
