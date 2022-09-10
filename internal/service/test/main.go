// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// + build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"changkun.de/x/midgard/internal/service"
)

var log service.Logger

func main() {
	var name = "test"
	var displayName = "test is test service"
	var desc = "test service tests description"
	var args = []string{"run"}

	var s, err = service.NewService(name, displayName, desc, args)
	log = s

	if err != nil {
		fmt.Printf("%s unable to start: %s", name, err)
		return
	}
	if len(os.Args) < 2 {
		fmt.Printf("%s unable to start: args not enough", name)
		return
	}

	defer func() {
		if err != nil {
			fmt.Printf("failed to %s, err: %v", os.Args[1], err)
			return
		}
		fmt.Printf("%s action is done.", os.Args[1])
	}()
	verb := os.Args[1]
	switch verb {
	case "install":
		err = s.Install()
	case "uninstall":
		err = s.Remove()
	case "start":
		err = s.Start()
	case "stop":
		err = s.Stop()
	case "run":
		err = s.Run(new(work).run(context.Background()))
	default:
		err = fmt.Errorf("%s is not a valid action", verb)
	}
}

type work struct{}

func (w *work) run(ctx context.Context) (onStart, onStop func() error) {
	ctx, cancel := context.WithCancel(ctx)
	onStart = func() error {
		go w.work(ctx)
		return nil
	}
	onStop = func() error {
		log.Info("Stopping!")
		cancel()
		return nil
	}
	return
}

func (w *work) work(ctx context.Context) {
	log.Info("Running!")
	ticker := time.NewTicker(time.Second * 10)
	for {
		select {
		case <-ticker.C:
			log.Info("Still running...")
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}
