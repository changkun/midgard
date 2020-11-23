// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

// +build freebsd linux netbsd openbsd solaris dragonfly

package clipboard

import (
	"bytes"
	"context"
	"os/exec"
	"time"
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

func read(t DataType) (buf []byte) {
	cmds := make([]string, len(pasteCmdArgs))
	copy(cmds, pasteCmdArgs)
	if t == DataTypeImagePNG {
		cmds = append(cmds, "-t", "image/png")
	}
	pasteCmd := exec.Command(cmds[0], cmds[1:]...)
	out, err := pasteCmd.CombinedOutput()
	if err != nil {
		return nil
	}
	return out
}
func write(buf []byte, t DataType) (ret bool) {
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

func watch(ctx context.Context, dt DataType, dataCh chan []byte) {
	// FIXME: this is not the ideal approach. On linux, we can interact
	// with X11 ICCCM to listen to the selection notification event,
	// then trigger the watch as needed to avoid frequent Read().
	t := time.NewTicker(time.Second)
	last := Read()
	for {
		select {
		case <-ctx.Done():
			close(dataCh)
			return
		case <-t.C:
			b := read(dt)
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
