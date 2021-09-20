// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package types

type OfficeStatusType int

const (
	OfficeStatusStandard OfficeStatusType = iota
	officeStatusCustom
)

type OfficeStatusRequest struct {
	Type    OfficeStatusType `json:"type"`
	Working bool             `json:"working"`
	Meeting bool             `json:"meeting"`
	Message string           `json:"message"`
}

type OfficeStatusResponse struct {
	Message string `json:"message"`
}
