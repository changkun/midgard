// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"time"

	"changkun.de/x/midgard/internal/types/proto"
)

// Connect connects to a midgard client
func Connect(callback func(ctx context.Context, c proto.MidgardClient)) {
	// No authentication: the daemon listens on a socket only this user
	// can reach (see rpcSocket).
	conn, err := dialRPC()
	if err != nil {
		fatal("cannot connect to the midgard daemon", "err", err)
	}
	defer conn.Close()
	client := proto.NewMidgardClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	callback(ctx, client)
}
