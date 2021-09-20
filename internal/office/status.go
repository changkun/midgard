// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package office

import (
	"fmt"
	"sync"
	"time"
)

type StatusType = int32

const (
	StatusUnknown StatusType = iota
	StatusOn
	StatusOff
	StatusVacation
)

type Status struct {
	sync.RWMutex

	System  string `json:"os"`
	Working bool   `json:"working"`
	Meeting bool   `json:"meeting"`

	Status        StatusType `json:"-"`
	LastAvailable time.Time  `json:"-"` // when did he leave?

	// TODO: predict return time.
	EstimateReturn time.Time `json:"-"` // when will he be back?
}

func NewStatus() *Status {
	s := &Status{
		Status:         StatusUnknown,
		LastAvailable:  time.Now(),
		EstimateReturn: time.Now().Add(time.Hour), // default back in an hour.
	}
	return s
}

var (
	unknownMessage = "Sorry, I can't tell you at the moment."
	htmlTmpl       = `<div id="office">Is Changkun in the office?
		<p id="office-status" style="color: %s;">%s</p>
	</div>`
)

func (s *Status) HTML() string {
	s.RLock()
	defer s.RUnlock()

	var message string

	// now update display message
	switch s.Status {
	case StatusUnknown:
		message = fmt.Sprintf(htmlTmpl, "gray", unknownMessage)
	case StatusOn:
		if !s.Meeting {
			message = fmt.Sprintf(htmlTmpl, "green", "Yes!")
		} else {
			message = fmt.Sprintf(htmlTmpl, "brown", "Yes! But current in a meeting.")
		}
	case StatusOff:
		message = fmt.Sprintf(htmlTmpl, "tomato",
			fmt.Sprintf("No, he left %s ago.", time.Since(s.LastAvailable).Round(time.Second)))
	case StatusVacation:
		if time.Since(s.EstimateReturn) > 0 {
			// date invalid
			message = fmt.Sprintf(htmlTmpl, "tomato",
				"No, he is on vacation and will return soon.")
		} else {
			var date string
			if s.EstimateReturn.Year() == time.Now().Year() { // same year
				date = s.EstimateReturn.Format("02 Jan")
			} else {
				date = s.EstimateReturn.Format("Jan 2, 2006")
			}
			message = fmt.Sprintf(htmlTmpl, "tomato",
				fmt.Sprintf("No, he is on vacation and will return at %v", date))
		}
	}

	return message
}

func (s *Status) String() string {
	s.RLock()
	defer s.RUnlock()

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
	switch s.Status {
	case StatusUnknown:
		message = unknownMessage
	case StatusOn:
		if !s.Meeting {
			message = "Yes!"
		} else {
			message = "Yes! But current in a meeting."
		}
	case StatusOff:
		message = fmt.Sprintf("No, he left %s ago.", time.Since(s.LastAvailable).Round(time.Second))
	case StatusVacation:
		if time.Since(s.EstimateReturn) > 0 {
			// date invalid
			message = "No, he is on vacation and will return soon."
		} else {
			var date string
			if s.EstimateReturn.Year() == time.Now().Year() { // same year
				date = s.EstimateReturn.Format("02 Jan")
			} else {
				date = s.EstimateReturn.Format("Jan 2, 2006")
			}
			message = fmt.Sprintf("No, he is on vacation and will return at %v", date)
		}
	}

	return message
}

type UpdateOption func(s *Status)

func Working(working bool) func(s *Status) {
	return func(s *Status) {
		var newStatus StatusType
		if working {
			newStatus = StatusOn
		} else {
			newStatus = StatusOff
			s.Meeting = false
		}

		oldStatus := s.Status
		s.Status = newStatus
		switch {
		case oldStatus == StatusOn && newStatus == StatusOff:
			s.LastAvailable = time.Now()
		case oldStatus == StatusOn && newStatus == StatusVacation:
			s.LastAvailable = time.Now()
		case oldStatus == StatusOn && newStatus == StatusOn:
			// nothing
		case oldStatus == StatusOff && newStatus == StatusOn:
			// nothing
		case oldStatus == StatusOff && newStatus == StatusVacation:
			// nothing
		case oldStatus == StatusOff && newStatus == StatusOff:
			// nothing
		case oldStatus == StatusVacation && newStatus == StatusOn:
			// nothing
		case oldStatus == StatusVacation && newStatus == StatusOff:
			// nothing
		case oldStatus == StatusVacation && newStatus == StatusVacation:
			// nothing
		default:
			// nothing
		}
	}
}

func Meeting(meeting bool) func(s *Status) {
	return func(s *Status) {
		s.Meeting = meeting
	}
}

func (s *Status) Update(opts ...UpdateOption) {
	s.Lock()
	defer s.Unlock()

	for _, opt := range opts {
		opt(s)
	}
}
