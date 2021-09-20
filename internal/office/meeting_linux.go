// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package office

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func IsInMeeting() (bool, error) {
	// TODO:
	return false, nil
}

func IsScreenLocked() (bool, error) {
	var (
		out    bytes.Buffer
		outErr bytes.Buffer
	)

	// Do this command:
	// gnome-screensaver-command -q | grep "is active"
	cmd := exec.Command("gnome-screensaver-command", "-q")
	cmd.Stdout = &out
	cmd.Stderr = &outErr
	err := cmd.Run()
	if err != nil {
		return false, fmt.Errorf("%w: %v", err, outErr.String())
	}
	if !strings.Contains(out.String(), "is active") {
		return false, nil
	}
	return true, nil
}
