// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"changkun.de/x/midgard/api/serv"
	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/term"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/types/proto"
	"changkun.de/x/midgard/internal/utils"
	"changkun.de/x/midgard/internal/version"
	"changkun.de/x/midgard/systray"
	"google.golang.org/grpc/status"
)

func Start() {
	systray.Run(func() {
		systray.SetTemplateIcon([]byte{0xe2, 0x9b, 0xb0, 0xef, 0xb8, 0x8f}, []byte{0xe2, 0x9b, 0xb0, 0xef, 0xb8, 0x8f})
		systray.SetTitle("⛰️ midgard")
		systray.SetTooltip("Universal Clipboard Service")
		ver := systray.AddMenuItem("Midgard"+version.String(), "")
		ver.Disable()

		systray.AddSeparator()

		s, d := getServerStatus(), getDaemonStatus()
		status := systray.AddMenuItem("Status", "Midgard status")
		servStatus := status.AddSubMenuItem(s, "")
		daemStatus := status.AddSubMenuItem(d, "")
		lists := getDevices()
		devices := systray.AddMenuItem("Devices", "Midgard devices")
		for _, list := range lists {
			devices.AddSubMenuItem(list, "")
		}
		systray.AddSeparator()
		clip := systray.AddMenuItem("Clipboard", "")
		systray.AddSeparator()
		q := systray.AddMenuItem("Exit", "Exit the daemon")

		for {
			select {
			case <-clip.ClickedCh:
				t, b := clipboard.Local.Read()
				switch t {
				case types.MIMEPlainText:
					fmt.Println("Clipboard content:")
					fmt.Println(string(b))
				case types.MIMEImagePNG:
					id, err := utils.NewUUIDShort()
					if err != nil {
						fmt.Printf("cannot create image filanem: %v\n", err)
						continue
					}
					os.WriteFile(id+".png", b, fs.ModePerm)
				}
			case <-servStatus.ClickedCh:
				servStatus.SetTitle(getServerStatus())
			case <-daemStatus.ClickedCh:
				daemStatus.SetTitle(getDaemonStatus())
			case <-q.ClickedCh:
				systray.Quit()
			}
		}
	}, func() {
		now := time.Now()
		ioutil.WriteFile(fmt.Sprintf(`on_exit_%d.txt`, now.UnixNano()), []byte(now.String()), 0644)
	})
}

func getDaemonStatus() (d string) {
	Connect(func(ctx context.Context, c proto.MidgardClient) {
		_, err := c.Ping(ctx, &proto.PingInput{})
		if err != nil {
			d += fmt.Sprintf("daemon status: %s, details:\n%v\n",
				"failed to ping daemon", status.Convert(err).Message())
		} else {
			d += fmt.Sprintf("daemon status: %s\n", "OK")
		}
	})
	return
}

func getServerStatus() (s string) {
	res, err := serv.Connect(http.MethodGet,
		config.Get().Domain+"/midgard/ping", nil)
	if err != nil {
		s += fmt.Sprintf("server status: %s, %v\n",
			term.Red("request error"), err)
	} else {
		var out types.PingOutput
		err = json.Unmarshal(res, &out)
		if err != nil {
			s += fmt.Sprintf("server status: %s, details:\n%v\n",
				"failed to parse ping response from server", err)
		} else {
			s += fmt.Sprintf("server status: %s\n", "OK")
		}
	}
	return s
}

func getDevices() []string {
	var ret []string
	Connect(func(ctx context.Context, c proto.MidgardClient) {
		out, err := c.ListDaemons(ctx, &proto.ListDaemonsInput{})
		if err != nil {
			log.Println("cannot list daemons:", status.Convert(err).Message())
			return
		}
		ret = append(ret, "id\tname\n")
		for i := 0; i < len(out.Id); i++ {
			ret = append(ret, fmt.Sprintf("%d\t%v\n", out.Id[i], out.Daemons[i]))
		}
	})
	return ret
}
