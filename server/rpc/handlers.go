// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rpc

import (
	"context"

	"golang.design/x/midgard/types/proto"
)

// Server implements midgard protobuf protocol
type Server struct{}

// Ping response a pong
func (s *Server) Ping(ctx context.Context, in *proto.PingInput) (*proto.PingOutput, error) {
	return &proto.PingOutput{}, nil
}

// GetClipboard ...
func (s *Server) GetClipboard(context.Context, *proto.GetClipboardInput) (*proto.GetClipboardOutput, error) {
	panic("unimplemented")
}

// PutClipboard ...
func (s *Server) PutClipboard(context.Context, *proto.PutClipboardInput) (*proto.PutClipboardOutput, error) {
	panic("unimplemented")
}

// GetURI ...
func (s *Server) GetURI(context.Context, *proto.GetURIInput) (*proto.GetURIOutput, error) {
	panic("unimplemented")
}
