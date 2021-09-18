package daemon

import (
	"context"
	"log"
	"time"

	"changkun.de/x/midgard/internal/types"
)

func (m *Daemon) watchOfficeStatus(ctx context.Context) {
	tk := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			log.Println("monitoring office status")

			// TODO: Do we need read message from server?
			// readerCh := make(chan *types.WebsocketMessage)
			// m.readChs.Store(m.ID, readerCh)

			m.writeCh <- &types.WebsocketMessage{
				Action:  types.ActionUpdateOfficeStatusRequest,
				UserID:  m.ID,
				Message: "office status has changed",
				Data:    nil,
			}
		}
	}
}
