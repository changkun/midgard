// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

// +build !(freebsd,linux,netbsd,openbsd,solaris,dragonfly,darwin)

package clipboard

import "context"

func read(t resType) (buf []byte, ok bool)          { panic("unimplemented") }
func write(buf []byte, t resType) (ret bool)        { panic("unimplemented") }
func watch(ctx context.Context, dataCh chan []byte) { panic("unimplemented") }
