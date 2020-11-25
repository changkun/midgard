// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rpc

import (
	"context"

	"golang.design/x/midgard/pkg/types/proto"
)

// Server implements midgard protobuf protocol
type Server struct{}

// Ping response a pong
func (s *Server) Ping(ctx context.Context, in *proto.PingInput) (*proto.PingOutput, error) {
	return &proto.PingOutput{Message: "pong"}, nil
}

// GetFromUniversalClipboard ...
func (s *Server) GetFromUniversalClipboard(context.Context, *proto.GetFromUniversalClipboardInput) (*proto.GetFromUniversalClipboardOutput, error) {
	panic("unimplemented")
}

// PutToUniversalClipboard ...
func (s *Server) PutToUniversalClipboard(context.Context, *proto.PutToUniversalClipboardInput) (*proto.PutToUniversalClipboardOutput, error) {
	panic("unimplemented")
}

// AllocateURL ...
func (s *Server) AllocateURL(context.Context, *proto.AllocateURLInput) (*proto.AllocateURLOutput, error) {
	panic("unimplemented")
}
