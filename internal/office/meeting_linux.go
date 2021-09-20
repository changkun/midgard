// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package office

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

func IsInMeeting() (bool, error) {
	// Do this command:
	// $ lsmod | grep uvcvideo
	// uvcvideo               98304  0 # if camera is off
	// uvcvideo               98304  1 # if camera is on

	cmd := exec.Command("lsmod")
	b, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	re := regexp.MustCompile(`uvcvideo(.*)`)
	match := re.FindStringSubmatch(string(b))

	if len(match) < 1 {
		return false, nil
	}
	matches := strings.Fields(match[1])
	if len(matches) < 1 {
		return false, nil
	}

	return matches[1] == "1", nil
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
