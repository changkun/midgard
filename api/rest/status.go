package rest

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TODO: think about how daemons can update the status here.

// Office returns the reported office status
func (m *Midgard) Office(c *gin.Context) {
	c.String(http.StatusOK, m.status.String())
}

func (m *Midgard) refreshStatus(ctx context.Context) {
	tk := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-tk.C:
			m.mu.Lock()
			// 1. No devices is connected to midgard, meaning offline
			if m.users.Len() == 0 {
				m.status.Update(statusOff)
			}
			// 2. If there are devices, waiting for them to report
			// current status, midgard server don't do anything.
			m.mu.Unlock()
		case <-ctx.Done():
			log.Println("status updater is terminated.")
			return
		}
	}
}

var unknownMessage = "Sorry, I can't tell you at the moment."

type officeStatus = int32

const (
	statusUnknown officeStatus = iota
	statusOn
	statusOff
	statusVacation
)

type myStatus struct {
	mu            sync.RWMutex
	status        officeStatus
	lastAvailable time.Time
	// TODO: vacation mode
}

func newMyStatus() *myStatus {
	return &myStatus{
		status:        statusUnknown,
		lastAvailable: time.Now(),
	}
}

func (s *myStatus) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// TODO: think about what type of status information I'd like to let
	// others to know

	// 1. current working machine (which system are you working on?)
	// 2. either on a meeting or not (are you in a meeting?)
	// 3. be able to update to a custom message (can I update my status using an iOS shortcut?)
	// 4. last avaliable time (when did you stopped working?)
	// 5. vacation (are you in a long term vacation?)
	// 6. anymore?

	var message string

	// now update display message
	switch s.status {
	case statusUnknown:
		message = unknownMessage
	case statusOn:
		message = "Yes!"
	case statusOff:
		message = fmt.Sprintf("No, he left %s ago.", time.Since(s.lastAvailable).Round(time.Second))
	case statusVacation:
		message = fmt.Sprintf("No, he is on vacation and will return soon.")
	}

	return message
}

func (s *myStatus) Update(status officeStatus) {
	defer log.Println("office status updated.")
	s.mu.Lock()
	defer s.mu.Unlock()

	oldStatus := s.status
	s.status = status

	// from On to Off
	if oldStatus == statusOn && status == statusOff {
		s.lastAvailable = time.Now()
	}

	// TODO: more state machine.
}
