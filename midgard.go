// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"golang.design/x/midgard/config"
	"golang.design/x/midgard/daemon"
	"golang.design/x/midgard/server"
)

var (
	// server options
	serv = flag.Bool("s", false, "run midgard server")

	// client deamon options
	daem = flag.Bool("d", false, "run midgard daemon")

	// client cli options
	genpath  = flag.String("p", "", "a specified uri for persistent")
	fromfile = flag.String("f", "", "attach data from file")
	// interactive = flag.String("i", "", "interactively input content")
)

func usage() {
	fmt.Fprintf(os.Stderr, `usage: midgard [-s] [-d]
options:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `example:
`)
	os.Exit(2)
}

func main() {
	log.SetPrefix(config.Get().Log.Prefix)
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)
	flag.Usage = usage
	flag.Parse()

	if *serv {
		server.Run()
		return
	}

	if *daem {
		daemon.Run()
		return
	}

	requestURI()
}
