// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package device

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"changkun.de/x/midgard/internal/signin"
	"changkun.de/x/midgard/internal/types"
	"github.com/gorilla/websocket"
)

// Dial connects to the configured server's websocket, signed in: what an
// Engine's Dial is, in the daemon and the Mac app alike.
func Dial(ctx context.Context) (*websocket.Conn, error) {
	return DialURL(ctx, types.EndpointSubscribe())
}

// DialURL connects to the websocket at url, signed in.
func DialURL(ctx context.Context, url string) (*websocket.Conn, error) {
	auth, err := signin.Authorization(ctx)
	if err != nil {
		return nil, err
	}
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, url, http.Header{"Authorization": {auth}})
	if err != nil {
		if notOnList(resp) {
			return nil, ErrNotOnList
		}
		return nil, fmt.Errorf("failed to connect midgard server: %w", err)
	}
	return conn, nil
}

// notOnList reports whether resp, a refused handshake's, says the server's
// allowlist does not have the one who signed in.
func notOnList(resp *http.Response) bool {
	if resp == nil || resp.StatusCode != http.StatusForbidden || resp.Body == nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	return strings.Contains(string(body), types.MsgNotOnList)
}

// ErrNotOnList is a server that knows who signed in, and does not let them
// in: its allowlist does not name them, or names them by an email it does
// not know is theirs until they sign in on its web page.
var ErrNotOnList = errors.New("the server's list does not have you: ask whoever runs it to add you, or, if they added your email, sign in once on its web page")
