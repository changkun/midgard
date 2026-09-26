// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"cmp"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"changkun.de/x/midgard/internal/config"
	"github.com/gin-gonic/gin"
	"latere.ai/x/pkg/authkit"
	"latere.ai/x/pkg/authkit/oidc"
)

// The web page and its sign-in live under the prefix. The sign-in routes
// start with a dot, so no share can be named like them.
const (
	webPage     = "/midgard/"
	webLogin    = "/midgard/.auth/login"
	webCallback = "/midgard/.auth/callback"
	webLogout   = "/midgard/.auth/logout"
)

// webSessionTTL is how long a browser stays signed in. Its access token is
// refreshed as it runs out; the allowlist is checked on every request, so
// someone taken off it is out at once.
const webSessionTTL = 30 * 24 * time.Hour

// webAuth signs people in from a browser, for the web page: the
// authorization code flow with PKCE through auth.latere.ai, the result kept
// in an encrypted cookie (specs/redesign.md §4, §7).
type webAuth struct {
	client *oidc.Client
	csrf   string // the name of the cookie that holds the page's CSRF token
	secure bool   // whether cookies are sent over https only
}

// newWebAuth sets up browser sign-in from the environment, as
// changkun.de's other services do: AUTH_CLIENT_ID and AUTH_COOKIE_KEY, and
// AUTH_REDIRECT_URL when the callback is not at the configured domain. It
// is nil when they are not set; the API works without it.
func newWebAuth() *webAuth {
	cfg := oidc.LoadConfig()
	if cfg.ClientID == "" {
		return nil
	}
	cfg.RedirectURL = cmp.Or(cfg.RedirectURL, config.ServerURL()+webCallback)
	if len(cfg.Scopes) == 0 {
		// offline_access for a refresh token, or the session would end
		// with its first access token
		cfg.Scopes = []string{"openid", "email", "profile", "offline_access"}
	}
	// Not the default name: redir signs in on the same site, and the two
	// would take each other's session.
	cfg.CookieName = "__Host-midgard-session"
	cfg.SessionTTL = webSessionTTL
	client := oidc.New(cfg)
	if client == nil {
		slog.Error("web sign-in is not set up: it needs AUTH_COOKIE_KEY", "client_id", cfg.ClientID)
		return nil
	}
	a := &webAuth{client: client, csrf: "__Host-midgard-csrf", secure: !cfg.InsecureCookies}
	if !a.secure {
		a.csrf = "midgard-csrf" // a __Host- cookie must be Secure
	}
	return a
}

// session reports who the session cookie in r signs in, if anyone. A
// cookie that cannot be read, or whose session is over, is cleared.
func (a *webAuth) session(w http.ResponseWriter, r *http.Request) (sub, email string, ok bool) {
	if a == nil {
		return "", "", false
	}
	sess, err := a.client.SessionFromRequest(w, r)
	if err != nil {
		if _, err := r.Cookie(a.cookieName()); err == nil {
			a.client.ClearSession(w) // there was one, and it is no good
		}
		return "", "", false
	}
	if sess.User.Sub == "" {
		return "", "", false
	}
	return sess.User.Sub, sess.User.Email, true
}

// cookieName is the session cookie's name as the browser has it.
func (a *webAuth) cookieName() string {
	if a.secure {
		return "__Host-midgard-session"
	}
	return "midgard-session"
}

// checkCSRF reports whether r carries, in its X-CSRF-Token header, the CSRF
// token of the page that sent it. Only the page can: the token is in a
// cookie only the site's own pages can read.
func (a *webAuth) checkCSRF(r *http.Request) bool {
	return a != nil && r.Header.Get(authkit.CSRFHeaderName()) != "" && authkit.CSRFValidate(r, a.csrf)
}

// csrfToken is the page's CSRF token: the one the browser has, so pages
// open in other tabs keep working, or a new one.
func (a *webAuth) csrfToken(w http.ResponseWriter, r *http.Request) string {
	if ck, err := r.Cookie(a.csrf); err == nil && ck.Value != "" {
		return ck.Value
	}
	return authkit.CSRFIssue(w, a.csrf, a.secure)
}

// login starts a sign-in that returns to the web page.
func (a *webAuth) login(c *gin.Context) {
	q := c.Request.URL.Query()
	q.Set("return_to", webPage)
	c.Request.URL.RawQuery = q.Encode()
	a.client.HandleLogin(c.Writer, c.Request)
}

// callback finishes a sign-in. The library sends a failed one to /login or
// /?auth_error=, the site's own pages, which on a shared site are not
// midgard's; they are sent to the web page instead.
func (a *webAuth) callback(c *gin.Context) {
	a.client.HandleCallback(&underPrefix{ResponseWriter: c.Writer}, c.Request)
}

// logout signs out, of midgard and of auth.latere.ai, and returns to the
// web page.
func (a *webAuth) logout(c *gin.Context) {
	q := c.Request.URL.Query()
	q.Set("return_to", webPage)
	c.Request.URL.RawQuery = q.Encode()
	a.client.HandleLogout(c.Writer, c.Request)
}

// underPrefix moves the redirects the sign-in library makes to the site's
// root under the web page.
type underPrefix struct{ http.ResponseWriter }

func (w *underPrefix) WriteHeader(code int) {
	h := w.Header()
	switch loc := h.Get("Location"); {
	case loc == "/login":
		h.Set("Location", webPage)
	case strings.HasPrefix(loc, "/?"):
		h.Set("Location", webPage+loc[1:])
	}
	w.ResponseWriter.WriteHeader(code)
}

//go:embed web/index.html
var indexHTML string

// The iPhone Shortcuts, built and signed by shortcuts/make.py: signed for
// anyone, so a phone adds one from a link.
//
//go:embed web/shortcuts/*.shortcut
var shortcutFiles embed.FS

// shortcutNames are the files the page offers, by the names a phone saves
// them under.
var shortcutNames = map[string]string{
	"get-from-midgard.shortcut": "Get from Midgard.shortcut",
	"send-to-midgard.shortcut":  "Send to Midgard.shortcut",
}

// Shortcut hands out one of the Shortcuts, to anyone: it holds no one's
// token, and asks for one when it is added.
func (m *Midgard) Shortcut(c *gin.Context) {
	name, ok := shortcutNames[c.Param("name")]
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	b, err := shortcutFiles.ReadFile("web/shortcuts/" + c.Param("name"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(http.StatusOK, "application/octet-stream", b)
}

var indexTmpl = template.Must(template.New("index").Parse(indexHTML))

// page is what the web page is rendered with.
type page struct {
	Nonce     string // for its one script and one stylesheet
	CSRF      string // sent back with every change it asks for
	Email     string // who is signed in; empty when no one is
	Allowed   bool   // whether they may use this server
	NoSignIn  bool   // this server has no web sign-in set up
	AuthError string // why the last sign-in failed, if it did
	Login     string
	Logout    string
	API       string
}

// WebPage serves the web page: one's clipboard, history, shares and app
// tokens, once signed in.
func (m *Midgard) WebPage(c *gin.Context) {
	p := page{
		Nonce:     nonce(),
		NoSignIn:  m.web == nil,
		AuthError: c.Query("auth_error"),
		Login:     webLogin,
		Logout:    webLogout,
		API:       "/midgard/api/v1",
	}
	if m.web != nil {
		p.CSRF = m.web.csrfToken(c.Writer, c.Request)
		if sub, email, ok := m.web.session(c.Writer, c.Request); ok {
			p.Email = cmp.Or(email, sub)
			p.Allowed = m.latere.permits(sub, email)
		}
	}

	h := c.Writer.Header()
	h.Set("Content-Security-Policy", strings.Join([]string{
		"default-src 'none'",
		"script-src 'nonce-" + p.Nonce + "'",
		"style-src 'nonce-" + p.Nonce + "'",
		"connect-src 'self'",
		"img-src 'self' data: blob:",
		"base-uri 'none'",
		"form-action 'none'",
		"frame-ancestors 'none'",
	}, "; "))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Cache-Control", "no-store") // it holds who is signed in, and the CSRF token
	h.Set("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	if err := indexTmpl.Execute(c.Writer, p); err != nil {
		slog.Error("cannot render the web page", "err", err)
	}
}

// nonce is a fresh value for the page's Content-Security-Policy.
func nonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b) // no + or /, which a template would escape
}
