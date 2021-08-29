// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"log"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types/proto"
	"google.golang.org/grpc"
)

// Connect connects to a midgard client
func Connect(callback func(ctx context.Context, c proto.MidgardClient)) {
	// We don't need authentication here. Daemon is running
	// on a local machine.
	conn, err := grpc.Dial(config.D().Addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: \n\t%v", err)
	}
	defer conn.Close()
	client := proto.NewMidgardClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	callback(ctx, client)
}
