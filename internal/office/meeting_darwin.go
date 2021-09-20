// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package office

/*
#cgo CFLAGS: -Werror -fmodules -fobjc-arc -x objective-c

#include <stdbool.h>

bool isScreenLocked();
bool isCameraOn();
*/
import "C"

func IsInMeeting() (bool, error) {
	return bool(C.isCameraOn()), nil
}

// IsScreenLocked returns true if the screen is locked, and false otherwise.
func IsScreenLocked() (bool, error) {
	return bool(C.isScreenLocked()), nil
}
