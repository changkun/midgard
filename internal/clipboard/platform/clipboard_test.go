// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package platform_test

import (
	"sync"
	"testing"

	"changkun.de/x/midgard/internal/clipboard/platform"
	"changkun.de/x/midgard/internal/types"
)

func TestLocalClipboardConcurrentRead(t *testing.T) {
	// This test check that concurrent read/write to the clipboard does
	// not cause crashes on some specific platform, such as macOS.
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		platform.Read(types.MIMEPlainText)
	}()
	go func() {
		defer wg.Done()
		platform.Read(types.MIMEImagePNG)
	}()
	wg.Wait()
}
