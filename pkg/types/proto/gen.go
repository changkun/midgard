// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

// +build none

//go:generate protoc --go_out=plugins=grpc:. midgard.proto

// Package proto defines the midgard gRPC protocols.
package proto
