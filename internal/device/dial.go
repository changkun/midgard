// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package device

import (
	"context"
	"fmt"
	"net/http"

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
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, http.Header{"Authorization": {auth}})
	if err != nil {
		return nil, fmt.Errorf("failed to connect midgard server: %w", err)
	}
	return conn, nil
}
