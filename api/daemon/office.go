// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"encoding/json"
	"log"
	"runtime"
	"time"

	"changkun.de/x/midgard/internal/office"
	"changkun.de/x/midgard/internal/types"
)

var s office.Status

func init() {
	switch runtime.GOOS {
	case "darwin":
		s.System = "macOS"
	case "linux":
		s.System = "Linux"
	case "windows":
		s.System = "Windows"
	default:
		s.System = "Unknown"
	}
}

func (m *Daemon) watchOfficeStatus(ctx context.Context) {
	tk := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			log.Println("monitoring office status")

			// Figure out the current office status
			locked, err := office.IsScreenLocked()
			if err != nil {
				locked = true
			}
			working := !locked
			meeting, err := office.IsInMeeting()
			if err != nil {
				meeting = false
			}

			// Check with local status and see if there are any updates
			updated := false
			s.Lock()
			if s.Meeting != meeting {
				updated = true
				s.Meeting = meeting
			}
			if s.Working != working {
				updated = true
				s.Working = working
			}
			s.Unlock()

			log.Printf("current status: working: %v, meeting %v", working, meeting)

			// do not communicate with server if there are no updates.
			if !updated {
				log.Println("office status has no updates.")
				continue
			}
			readerCh := make(chan *types.WebsocketMessage)
			m.readChs.Store(m.ID, readerCh)

			b, _ := json.Marshal(&types.OfficeStatusRequest{
				Type:    types.OfficeStatusStandard,
				Working: s.Working,
				Meeting: s.Meeting,
			})
			m.writeCh <- &types.WebsocketMessage{
				Action:  types.ActionUpdateOfficeStatusRequest,
				UserID:  m.ID,
				Message: "office status has changed",
				Data:    b,
			}

			resp := <-readerCh
			switch resp.Action {
			case types.ActionUpdateOfficeStatusResponse:
				var data types.OfficeStatusResponse
				err := json.Unmarshal(resp.Data, &data)
				if err != nil {
					log.Printf("failed to parse office status response, there must be a server side error: %v", err)
				}
				log.Println(data.Message)
				m.readChs.Delete(m.ID)
			default:
				// not interested, ingore.
			}
		}
	}
}
