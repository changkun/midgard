// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package daemon

import (
	"github.com/spf13/cobra"
	"golang.design/x/midgard/pkg/watch"
)

// Run runs the midgard's daemon process.
func Run(*cobra.Command, []string) {
	// TODO: we have several remaining task for the daemon:
	//
	// 1. register a websocket connection for universal clipboard push
	// notification: if the cloud is changed, then it should notify all
	// subscribers, instead of the following deadloop:
	//
	// go func() {
	// 	url := "http://" + config.Get().Addr.HTTP + "/midgard/api/v1/clipboard"
	// 	t := time.NewTicker(time.Second * 2)
	// 	for {
	// 		select {
	// 		case <-t.C:
	// 			_, err := request(http.MethodGet, url, nil)
	// 			if err != nil {
	// 				fmt.Println(err)
	// 			}
	// 		}
	// 	}
	// }()
	//
	// 2. register to system hotkey, trigger special handlers
	watch.Clipboard()
}
