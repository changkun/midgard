// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package device

import (
	"errors"
	"fmt"

	"changkun.de/x/midgard/internal/client"
	"changkun.de/x/midgard/internal/e2e"
)

// Pairing gives a new device its person's key (specs/redesign.md §11): a
// paired device shows a code and leaves the key, sealed under it, on the
// server; the new device, given the code, takes it.

// ShowPairing makes a pairing code for a new device of the person's, and
// leaves their key, sealed under it, on the server for ten minutes.
func ShowPairing(k *e2e.Key, since uint64) (e2e.Code, error) {
	code, err := e2e.NewCode()
	if err != nil {
		return e2e.Code{}, err
	}
	box, err := code.Seal(k, since)
	if err != nil {
		return e2e.Code{}, err
	}
	if err := client.LeavePairing(code.Mailbox(), box); err != nil {
		return e2e.Code{}, err
	}
	return code, nil
}

// JoinPairing takes the key the pairing code's box holds, from the server.
func JoinPairing(code e2e.Code) (*e2e.Key, uint64, error) {
	box, err := client.TakePairing(code.Mailbox())
	if errors.Is(err, client.ErrNotFound) {
		return nil, 0, fmt.Errorf("no pairing waits for this code: it was used, or ten minutes passed; show a new one (%w)", err)
	}
	if err != nil {
		return nil, 0, err
	}
	k, since, err := code.Open(box)
	if err != nil {
		return nil, 0, fmt.Errorf("the code did not open what waited for it: %w", err)
	}
	return k, since, nil
}
