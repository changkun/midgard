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

var s office.LocalStatus

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
			b, _ := json.Marshal(&s)
			m.writeCh <- &types.WebsocketMessage{
				Action:  types.ActionUpdateOfficeStatusRequest,
				UserID:  m.ID,
				Message: "office status has changed",
				Data:    b,
			}
		}
	}
}

// TODO: Do we need read message from server?
// readerCh := make(chan *types.WebsocketMessage)
// m.readChs.Store(m.ID, readerCh)
