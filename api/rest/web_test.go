// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"latere.ai/x/pkg/authkit"
	"latere.ai/x/pkg/authkit/oidc"
)

// withWeb sets m up for sign-in from a browser, against testIssuer.
func withWeb(t *testing.T, m *Midgard) *webAuth {
	t.Helper()
	t.Setenv("AUTH_CLIENT_ID", "midgard-web")
	t.Setenv("AUTH_COOKIE_KEY", strings.Repeat("ab", 32))
	t.Setenv("AUTH_REDIRECT_URL", "https://example.com/midgard/.auth/callback")
	m.web = newWebAuth()
	if m.web == nil {
		t.Fatal("web sign-in is not set up")
	}
	return m.web
}

// sessionFor is the session cookie of someone signed in on the web page.
func sessionFor(t *testing.T, a *webAuth, sub, email string) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	err := a.client.SetSession(rec, &oidc.Session{
		AccessToken: "an access token", Expiry: time.Now().Add(time.Hour),
		User: oidc.User{Identity: authkit.Identity{Sub: sub, Email: email}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return rec.Result().Cookies()[0]
}

// csrf is a CSRF cookie with the token it holds.
func csrf(a *webAuth) (*http.Cookie, string) {
	return &http.Cookie{Name: a.csrf, Value: "the-page-token"}, "the-page-token"
}

// browse sends a request as a browser would: with cookies, and headers.
func browse(t *testing.T, m *Midgard, method, path, body string, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	m.routers().ServeHTTP(w, req)
	return w
}

// nonceOf is the nonce a page's Content-Security-Policy allows scripts by.
var nonceOf = regexp.MustCompile(`script-src 'nonce-([^']+)'`)

func TestWebPage(t *testing.T) {
	resetBlocklist(t)
	t.Run("no web sign-in", func(t *testing.T) {
		w := do(t, testMidgard(t), http.MethodGet, "/midgard/", "", false)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "not set up on this server") {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})

	m := testMidgard(t)
	a := withWeb(t, m)
	t.Run("signed out", func(t *testing.T) {
		w := browse(t, m, http.MethodGet, "/midgard/", "", nil, nil)
		body := w.Body.String()
		if w.Code != http.StatusOK || !strings.Contains(body, `href="/midgard/.auth/login"`) || strings.Contains(body, "data-signed-in") {
			t.Fatalf("%d %s, want a way to sign in", w.Code, body)
		}
		// a fresh CSRF token, in a cookie and in the page
		var tok string
		for _, c := range w.Result().Cookies() {
			if c.Name == a.csrf {
				tok = c.Value
			}
		}
		if tok == "" || !strings.Contains(body, `data-csrf="`+tok+`"`) {
			t.Errorf("the page and its cookie do not share a CSRF token (%q)", tok)
		}
	})
	t.Run("headers", func(t *testing.T) {
		w := browse(t, m, http.MethodGet, "/midgard/", "", nil, nil)
		csp := w.Header().Get("Content-Security-Policy")
		n := nonceOf.FindStringSubmatch(csp)
		if n == nil || !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Fatalf("Content-Security-Policy = %q", csp)
		}
		if !strings.Contains(w.Body.String(), `<script nonce="`+n[1]+`">`) {
			t.Error("the page's script does not carry the policy's nonce")
		}
		if again := nonceOf.FindStringSubmatch(browse(t, m, http.MethodGet, "/midgard/", "", nil, nil).Header().Get("Content-Security-Policy")); again[1] == n[1] {
			t.Error("two pages have the same nonce")
		}
		for k, want := range map[string]string{"Cache-Control": "no-store", "X-Content-Type-Options": "nosniff", "Content-Type": "text/html; charset=utf-8"} {
			if got := w.Header().Get(k); got != want {
				t.Errorf("%s = %q, want %q", k, got, want)
			}
		}
	})
	t.Run("keeps the CSRF token of other tabs", func(t *testing.T) {
		ck, tok := csrf(a)
		w := browse(t, m, http.MethodGet, "/midgard/", "", []*http.Cookie{ck}, nil)
		if !strings.Contains(w.Body.String(), `data-csrf="`+tok+`"`) || len(w.Result().Cookies()) != 0 {
			t.Errorf("a page load replaced the CSRF token the browser had")
		}
	})
	t.Run("signed in", func(t *testing.T) {
		w := browse(t, m, http.MethodGet, "/midgard/", "", []*http.Cookie{sessionFor(t, a, testUser, testEmail)}, nil)
		body := w.Body.String()
		if !strings.Contains(body, testEmail) || !strings.Contains(body, "data-signed-in") || !strings.Contains(body, `href="/midgard/.auth/logout"`) {
			t.Fatalf("%d %s, want the signed-in page", w.Code, body)
		}
	})
	t.Run("not allowed", func(t *testing.T) {
		w := browse(t, m, http.MethodGet, "/midgard/", "", []*http.Cookie{sessionFor(t, a, "sub-eve", "eve@example.com")}, nil)
		body := w.Body.String()
		if !strings.Contains(body, "eve@example.com is not on its list") || !strings.Contains(body, "docs/install.md") || strings.Contains(body, "data-signed-in") {
			t.Fatalf("%s, want eve turned away", body)
		}
	})
	t.Run("escapes", func(t *testing.T) {
		w := browse(t, m, http.MethodGet, "/midgard/?auth_error=%3Cscript%3Ealert(1)%3C/script%3E", "",
			[]*http.Cookie{sessionFor(t, a, "sub-x", "<img src=x onerror=alert(1)>")}, nil)
		body := w.Body.String()
		if strings.Contains(body, "<script>alert") || strings.Contains(body, "<img src=x") {
			t.Fatalf("the page renders markup it was given:\n%s", body)
		}
	})
}

// TestWebSession: the page's cookie signs its owner in to the API, and no
// one else, and changes nothing without the page's CSRF token.
func TestWebSession(t *testing.T) {
	resetBlocklist(t)
	m := testMidgard(t)
	a := withWeb(t, m)
	if w := do(t, m, http.MethodPost, "/midgard/api/v1/clipboard", `{"type":"text","data":"mine"}`, true); w.Code != http.StatusOK {
		t.Fatalf("copying: %d %s", w.Code, w.Body)
	}
	session := sessionFor(t, a, testUser, testEmail)
	ck, tok := csrf(a)

	w := browse(t, m, http.MethodGet, "/midgard/api/v1/clipboard", "", []*http.Cookie{session}, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"mine"`) {
		t.Fatalf("reading with the session: %d %s", w.Code, w.Body)
	}
	if w := browse(t, m, http.MethodGet, "/midgard/api/v1/clipboard", "", []*http.Cookie{sessionFor(t, a, "sub-eve", "eve@example.com")}, nil); w.Code != http.StatusUnauthorized {
		t.Errorf("someone not on the allowlist: %d, want 401", w.Code)
	}
	bad := &http.Cookie{Name: session.Name, Value: "not a session"}
	w = browse(t, m, http.MethodGet, "/midgard/api/v1/clipboard", "", []*http.Cookie{bad}, nil)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Header().Get("Set-Cookie"), session.Name+"=;") {
		t.Errorf("a broken session: %d, Set-Cookie %q; want 401 and the cookie cleared", w.Code, w.Header().Get("Set-Cookie"))
	}

	put := `{"type":"text","data":"from the web"}`
	for _, tt := range []struct {
		name    string
		cookies []*http.Cookie
		headers map[string]string
		want    int
	}{
		{"no token", []*http.Cookie{session, ck}, nil, http.StatusForbidden},
		{"no cookie", []*http.Cookie{session}, map[string]string{"X-CSRF-Token": tok}, http.StatusForbidden},
		{"another token", []*http.Cookie{session, ck}, map[string]string{"X-CSRF-Token": "guessed"}, http.StatusForbidden},
		{"the page's token", []*http.Cookie{session, ck}, map[string]string{"X-CSRF-Token": tok}, http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if w := browse(t, m, http.MethodPost, "/midgard/api/v1/clipboard", put, tt.cookies, tt.headers); w.Code != tt.want {
				t.Fatalf("got %d %s, want %d", w.Code, w.Body, tt.want)
			}
		})
	}
	rm, _ := m.rel().room(context.Background(), testUser)
	if f, _ := rm.newest(); string(f.Payload) != "from the web" || f.Origin != "web" {
		t.Errorf("the clipboard is %q from %q, want the web page's copy", f.Payload, f.Origin)
	}
	// a token needs no CSRF token: no browser sends it on its own
	if w := do(t, m, http.MethodPost, "/midgard/api/v1/clipboard", put, true); w.Code != http.StatusOK {
		t.Errorf("a token without a CSRF token: %d", w.Code)
	}

	// the websocket and the profiles are not for the page
	for _, path := range []string{"/midgard/api/v1/ws", "/midgard/api/v1/debug/pprof/"} {
		if w := browse(t, m, http.MethodGet, path, "", []*http.Cookie{session}, nil); w.Code != http.StatusUnauthorized {
			t.Errorf("GET %s with the session: %d, want 401", path, w.Code)
		}
	}
	if w := do(t, m, http.MethodGet, "/midgard/api/v1/debug/pprof/", "", true); w.Code != http.StatusOK {
		t.Errorf("the profiles with a token: %d, want 200", w.Code)
	}
}

// TestTokenEndpoints: a person, signed in, issues and revokes their app
// tokens; an app token cannot.
func TestTokenEndpoints(t *testing.T) {
	resetBlocklist(t)
	m := testMidgard(t)
	a := withWeb(t, m)
	ck, tok := csrf(a)
	web := []*http.Cookie{sessionFor(t, a, testUser, testEmail), ck}
	hdr := map[string]string{"X-CSRF-Token": tok}

	w := browse(t, m, http.MethodPost, "/midgard/api/v1/tokens", `{"name":"phone"}`, web, hdr)
	var issued types.TokenInfo
	if json.Unmarshal(w.Body.Bytes(), &issued); w.Code != http.StatusOK || !strings.HasPrefix(issued.Token, "mgt_") || issued.Name != "phone" {
		t.Fatalf("issuing: %d %s", w.Code, w.Body)
	}
	if w := browse(t, m, http.MethodPost, "/midgard/api/v1/tokens", `{"name":"phone"}`, web, hdr); w.Code != http.StatusBadRequest {
		t.Errorf("a second token of the same name: %d, want 400", w.Code)
	}
	if w := browse(t, m, http.MethodPost, "/midgard/api/v1/tokens", `{"name":"no spaces"}`, web, hdr); w.Code != http.StatusBadRequest {
		t.Errorf("an invalid name: %d, want 400", w.Code)
	}

	w = browse(t, m, http.MethodGet, "/midgard/api/v1/tokens", "", web, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"phone"`) || strings.Contains(w.Body.String(), issued.Token) {
		t.Fatalf("listing: %d %s; want the name and not the token", w.Code, w.Body)
	}

	// the token works, and reaches its owner's data, as the one it was
	// issued with the email of
	app := map[string]string{"Authorization": "Bearer " + issued.Token}
	if w := browse(t, m, http.MethodGet, "/midgard/api/v1/devices", "", nil, app); w.Code != http.StatusOK {
		t.Fatalf("the issued token: %d", w.Code)
	}
	for _, r := range []struct{ method, path, body string }{
		{http.MethodGet, "/midgard/api/v1/tokens", ""},
		{http.MethodPost, "/midgard/api/v1/tokens", `{"name":"more"}`},
		{http.MethodDelete, "/midgard/api/v1/tokens/phone", ""},
	} {
		if w := browse(t, m, r.method, r.path, r.body, nil, app); w.Code != http.StatusForbidden {
			t.Errorf("%s %s with an app token: %d, want 403", r.method, r.path, w.Code)
		}
	}

	// a token from auth.latere.ai is a person signed in, too
	if w := do(t, m, http.MethodGet, "/midgard/api/v1/tokens", "", true); w.Code != http.StatusOK {
		t.Errorf("listing with a latere token: %d", w.Code)
	}

	if w := browse(t, m, http.MethodDelete, "/midgard/api/v1/tokens/phone", "", web, hdr); w.Code != http.StatusNoContent {
		t.Fatalf("revoking: %d %s", w.Code, w.Body)
	}
	if w := browse(t, m, http.MethodGet, "/midgard/api/v1/devices", "", nil, app); w.Code != http.StatusUnauthorized {
		t.Errorf("a revoked token: %d, want 401", w.Code)
	}
	if w := browse(t, m, http.MethodDelete, "/midgard/api/v1/tokens/phone", "", web, hdr); w.Code != http.StatusNotFound {
		t.Errorf("revoking it again: %d, want 404", w.Code)
	}
}

// TestWebSignIn: the sign-in routes start and end under the prefix, never
// at the site's root, which on changkun.de is another service.
func TestWebSignIn(t *testing.T) {
	resetBlocklist(t)
	m := testMidgard(t)
	a := withWeb(t, m)

	for _, returnTo := range []string{"", "/", "//evil.example", "/midgard/../elsewhere"} {
		w := browse(t, m, http.MethodGet, "/midgard/.auth/login?return_to="+url.QueryEscape(returnTo), "", nil, nil)
		loc, _ := url.Parse(w.Header().Get("Location"))
		q := loc.Query()
		if w.Code != http.StatusFound || !strings.HasPrefix(loc.String(), testIssuer.URL()+"/authorize") ||
			q.Get("client_id") != "midgard-web" || q.Get("redirect_uri") != "https://example.com/midgard/.auth/callback" ||
			q.Get("code_challenge_method") != "S256" || !strings.Contains(q.Get("scope"), "offline_access") {
			t.Fatalf("login: %d %s", w.Code, loc)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, c := range w.Result().Cookies() {
			req.AddCookie(c)
		}
		flow, err := a.client.GetFlowState(req)
		if err != nil || flow.ReturnTo != "/midgard/" {
			t.Errorf("return_to=%q: the sign-in returns to %+v, %v; want the web page", returnTo, flow, err)
		}
	}

	for _, tt := range []struct{ query, want string }{
		{"?error=access_denied", "/midgard/?auth_error=access_denied"},
		{"?code=x&state=y", "/midgard/"}, // no sign-in was started here
	} {
		w := browse(t, m, http.MethodGet, "/midgard/.auth/callback"+tt.query, "", nil, nil)
		if loc := w.Header().Get("Location"); w.Code != http.StatusFound || loc != tt.want {
			t.Errorf("callback%s: %d to %q, want %q", tt.query, w.Code, loc, tt.want)
		}
	}

	w := browse(t, m, http.MethodGet, "/midgard/.auth/logout", "", []*http.Cookie{sessionFor(t, a, testUser, testEmail)}, nil)
	want := testIssuer.URL() + "/logout?post_logout_redirect_uri=" + url.QueryEscape("https://example.com/midgard/")
	if loc := w.Header().Get("Location"); w.Code != http.StatusFound || loc != want {
		t.Errorf("logout: %d to %q, want %q", w.Code, loc, want)
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "__Host-midgard-session=;") {
		t.Errorf("logout left the session: %q", w.Header().Get("Set-Cookie"))
	}

	// without web sign-in there are no such routes
	t.Setenv("AUTH_CLIENT_ID", "")
	if w := do(t, testMidgard(t), http.MethodGet, "/midgard/.auth/login", "", false); w.Code != http.StatusNotFound {
		t.Errorf("login without web sign-in: %d, want 404", w.Code)
	}
}

// TestShortcuts: the iPhone Shortcuts are handed out to anyone, as files a
// phone saves under their names; nothing else is.
func TestShortcuts(t *testing.T) {
	m := testMidgard(t)
	for file, name := range shortcutNames {
		w := do(t, m, http.MethodGet, "/midgard/shortcuts/"+file, "", false)
		if w.Code != http.StatusOK || w.Body.Len() < 1000 || !strings.HasPrefix(w.Body.String(), "AEA1") {
			t.Fatalf("%s: %d, %d bytes", file, w.Code, w.Body.Len())
		}
		if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, name) {
			t.Errorf("%s: Content-Disposition %q", file, got)
		}
	}
	for _, path := range []string{"/midgard/shortcuts/nope.shortcut", "/midgard/shortcuts/..%2findex.html"} {
		if w := do(t, m, http.MethodGet, path, "", false); w.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", path, w.Code)
		}
	}
	// and no share can take their place
	if code, _ := share(t, m, types.ShareInput{Data: b64("x"), Name: "shortcuts/get-from-midgard.shortcut"}); code != http.StatusBadRequest {
		t.Errorf("a share named like a Shortcut: %d, want 400", code)
	}
}

// TestIcons: the page names its icons, and they are served, to anyone.
func TestIcons(t *testing.T) {
	m := testMidgard(t)
	page := do(t, m, http.MethodGet, "/midgard/", "", false).Body.String()
	for name, typ := range iconTypes {
		path := "/midgard/icons/" + name
		if !strings.Contains(page, `href="`+path+`"`) {
			t.Errorf("the page does not name %s", path)
		}
		w := do(t, m, http.MethodGet, path, "", false)
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != typ || w.Body.Len() < 1000 {
			t.Errorf("GET %s: %d, %q, %d bytes", path, w.Code, w.Header().Get("Content-Type"), w.Body.Len())
		}
	}
	if w := do(t, m, http.MethodGet, "/midgard/icons/nope.png", "", false); w.Code != http.StatusNotFound {
		t.Errorf("GET an icon there is not: %d, want 404", w.Code)
	}
	if code, _ := share(t, m, types.ShareInput{Data: b64("x"), Name: "icons/midgard.svg"}); code != http.StatusBadRequest {
		t.Errorf("a share named like an icon: %d, want 400", code)
	}
}

// TestImages: the front page shows its pictures, and they are served, to
// anyone.
func TestImages(t *testing.T) {
	m := testMidgard(t)
	page := do(t, m, http.MethodGet, "/midgard/", "", false).Body.String()
	for name, typ := range imageTypes {
		path := "/midgard/images/" + name
		if !strings.Contains(page, `src="`+path+`"`) {
			t.Errorf("the front page does not show %s", path)
		}
		w := do(t, m, http.MethodGet, path, "", false)
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != typ || w.Body.Len() < 1000 {
			t.Errorf("GET %s: %d, %q, %d bytes", path, w.Code, w.Header().Get("Content-Type"), w.Body.Len())
		}
	}
	if w := do(t, m, http.MethodGet, "/midgard/images/nope.png", "", false); w.Code != http.StatusNotFound {
		t.Errorf("GET a picture there is not: %d, want 404", w.Code)
	}
	if code, _ := share(t, m, types.ShareInput{Data: b64("x"), Name: "images/hero.svg"}); code != http.StatusBadRequest {
		t.Errorf("a share named like a picture: %d, want 400", code)
	}
}

// TestDownload: the Mac app is handed out to anyone once the server has it,
// and the page offers it only then; nothing else in the data folder is.
func TestDownload(t *testing.T) {
	resetBlocklist(t)
	t.Chdir(t.TempDir())
	m := testMidgard(t)
	a := withWeb(t, m)
	const path = "/midgard/download/Midgard.dmg"
	page := func(cookies ...*http.Cookie) string {
		return browse(t, m, http.MethodGet, "/midgard/", "", cookies, nil).Body.String()
	}

	if w := do(t, m, http.MethodGet, path, "", false); w.Code != http.StatusNotFound {
		t.Errorf("before there is an app: %d, want 404", w.Code)
	}
	if strings.Contains(page(), path) {
		t.Error("the page offers an app the server does not have")
	}

	app := strings.Repeat("koly", 1000)
	os.MkdirAll(config.DownloadPath, 0o755)
	os.WriteFile(filepath.Join(config.DownloadPath, "Midgard.dmg"), []byte(app), 0o644)
	os.WriteFile(filepath.Join(config.DownloadPath, "other.dmg"), []byte("x"), 0o644)

	w := do(t, m, http.MethodGet, path, "", false)
	if w.Code != http.StatusOK || w.Body.String() != app {
		t.Fatalf("GET %s: %d, %d bytes", path, w.Code, w.Body.Len())
	}
	for k, want := range map[string]string{
		"Content-Type":        "application/x-apple-diskimage",
		"Content-Disposition": `attachment; filename="Midgard.dmg"`,
		"Accept-Ranges":       "bytes", // so a broken download resumes
	} {
		if got := w.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if !strings.Contains(page(), `href="`+path+`"`) {
		t.Error("the signed-out page does not offer the app")
	}
	if body := page(sessionFor(t, a, testUser, testEmail)); !strings.Contains(body, `id="mac"`) || !strings.Contains(body, "drag Midgard to Applications") {
		t.Error("the signed-in page does not say how to install the app")
	}

	for _, p := range []string{"/midgard/download/other.dmg", "/midgard/download/..%2fdb%2fmidgard.db", "/midgard/download/"} {
		if w := do(t, m, http.MethodGet, p, "", false); w.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", p, w.Code)
		}
	}
	if code, _ := share(t, m, types.ShareInput{Data: b64("x"), Name: "download/Midgard.dmg"}); code != http.StatusBadRequest {
		t.Errorf("a share named like the app: %d, want 400", code)
	}
}
