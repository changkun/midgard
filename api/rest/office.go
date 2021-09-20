package rest

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"changkun.de/x/midgard/internal/office"
	"changkun.de/x/midgard/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Office returns the reported office status
func (m *Midgard) Office(c *gin.Context) {
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, m.status.HTML())
}

func (m *Midgard) refreshStatus(ctx context.Context) {
	tk := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-tk.C:
			m.mu.Lock()
			// 1. No devices is connected to midgard, meaning offline
			m.status.Update(office.Working(m.users.Len() != 0))

			// 2. If there are devices, waiting for them to report
			// current status, midgard server don't do anything.
			m.mu.Unlock()
		case <-ctx.Done():
			log.Println("status updater is terminated.")
			return
		}
	}
}

// handleOfficeStatusUpdate handles the update request from daemon.
func (m *Midgard) handleOfficeStatusUpdate(conn *websocket.Conn, u *user, data []byte) (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	defer func() {
		if err != nil {
			err = terminate(conn, err)
		}
	}()

	var s types.OfficeStatusRequest
	err = json.Unmarshal(data, &s)
	if err != nil {
		return
	}

	m.status.Update(office.Working(s.Working), office.Meeting(s.Meeting))
	b, _ := json.Marshal(&types.OfficeStatusResponse{
		Message: "Office status is updated.",
	})
	return conn.WriteMessage(websocket.BinaryMessage, (&types.WebsocketMessage{
		Action: types.ActionUpdateOfficeStatusResponse,
		Data:   b,
	}).Encode())
}
