// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// +build freebsd linux netbsd openbsd solaris dragonfly

package platform

import (
	"bytes"
	"context"
	"os/exec"
	"time"

	"changkun.de/x/midgard/internal/types"
)

var (
	xclip        = "xclip"
	pasteCmdArgs = []string{xclip, "-out", "-selection", "clipboard"}
	copyCmdArgs  = []string{xclip, "-in", "-selection", "clipboard"}
)

func init() {
	if _, err := exec.LookPath(xclip); err == nil {
		return
	}
	panic("please intall xclip on your system: sudo apt install xclip")
}

// Read reads the clipboard data of a given resource type.
// It returns a buf that containing the clipboard data, and ok indicates
// whether the read is success or fail.
func Read(t types.MIME) (buf []byte) {
	cmds := make([]string, len(pasteCmdArgs))
	copy(cmds, pasteCmdArgs)
	if t == types.MIMEImagePNG {
		cmds = append(cmds, "-t", "image/png")
	}
	pasteCmd := exec.Command(cmds[0], cmds[1:]...)
	out, err := pasteCmd.CombinedOutput()
	if err != nil {
		return nil
	}
	return out
}

// Write writes the given buf as typ to system clipboard.
// It returns true if the write is success.
func Write(buf []byte, t types.MIME) (ret bool) {
	copyCmd := exec.Command(copyCmdArgs[0], copyCmdArgs[1:]...)
	in, err := copyCmd.StdinPipe()
	if err != nil {
		return false
	}

	if err := copyCmd.Start(); err != nil {
		return false
	}
	if _, err := in.Write(buf); err != nil {
		return false
	}
	if err := in.Close(); err != nil {
		return false
	}
	if err := copyCmd.Wait(); err != nil {
		return false
	}
	return true
}

// Watch watches the changes of system clipboard, and sends the data of
// clipboard to the given dataCh.
func Watch(ctx context.Context, dt types.MIME, dataCh chan []byte) {
	// FIXME: this is not the ideal approach. On linux, we can interact
	// with X11 ICCCM to listen to the selection notification event,
	// then trigger the watch as needed to avoid frequent Read().
	t := time.NewTicker(time.Second)
	last := Read(dt)
	for {
		select {
		case <-ctx.Done():
			close(dataCh)
			return
		case <-t.C:
			b := Read(dt)
			if b == nil {
				continue
			}
			if bytes.Compare(last, b) != 0 {
				dataCh <- b
				last = b
			}
		}
	}
}
