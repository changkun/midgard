// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package server

import (
	"github.com/spf13/cobra"
	"golang.design/x/midgard/api"
)

// Run runs the midgard server.
func Run(*cobra.Command, []string) {
	m := api.NewMidgard()
	m.Serve()
}
