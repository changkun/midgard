// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package office

import "sync"

type LocalStatus struct {
	System string `json:"os"`

	sync.Mutex
	Working bool `json:"working"`
	Meeting bool `json:"meeting"`
}
