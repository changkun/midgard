// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"gopkg.in/yaml.v3"
)

// Clipboard is an interface that defines the operations of a clipboard
type Clipboard interface {
	// Read reads the clipboard and returns the MIME type and
	// the raw bytes data in the clipboard
	Read() (types.MIME, []byte)
	// Write write the given data as the given MIME type and
	// returns true if success or false if failed.
	Write(types.MIME, []byte) bool
}

// UniversalClipboard is an of Clipboard interface for universal purpose
type UniversalClipboard interface {
	Clipboard
	// ReadAs reads the clipboard as a given MIME type and return
	// the raw bytes if the type matches or nil if it does not.
	// This method is generally faster than the Clipboard.Read because
	// it avoids data copy if the MIME type does not match.
	ReadAs(t types.MIME) []byte
}

// UniversalFor is owner's universal clipboard on the server. It keeps the
// data in memory, and logs its change history to the data folder only when
// the configuration asks for it (server.store.log_clipboard).
//
// It holds a global shared storage that can be edited/fetched at anytime.
//
// There is one per person, and no shared one: a person reaches only theirs,
// by the owner their sign-in names (specs/redesign.md §5).
func UniversalFor(owner string) UniversalClipboard {
	v, _ := universals.LoadOrStore(owner, &universal{typ: types.MIMEPlainText, buf: []byte{}})
	return v.(*universal)
}

// universals holds each person's universal clipboard, by owner.
var universals sync.Map

type universal struct {
	sync.Mutex
	typ types.MIME
	buf []byte
}

func (uc *universal) Read() (types.MIME, []byte) {
	uc.Lock()
	defer uc.Unlock()
	buf := make([]byte, len(uc.buf))
	copy(buf, uc.buf)
	return uc.typ, buf
}

func (uc *universal) ReadAs(t types.MIME) []byte {
	uc.Lock()
	defer uc.Unlock()
	if t != uc.typ {
		return nil
	}

	buf := make([]byte, len(uc.buf))
	copy(buf, uc.buf)
	return buf
}

func (uc *universal) Write(t types.MIME, buf []byte) bool {
	uc.Lock()
	defer uc.Unlock()
	if uc.typ == t && bytes.Equal(uc.buf, buf) {
		return false
	}

	if config.S().Store.LogClipboard {
		uc.log(t, buf)
	}

	uc.typ = t
	uc.buf = buf
	return true
}

func (uc *universal) log(t types.MIME, buf []byte) {
	if t != types.MIMEPlainText {
		buf = utils.StringToBytes(string(t))
	}

	date := time.Now().UTC()
	r := struct {
		Time time.Time
		Type types.MIME
		Data string
	}{
		Time: date,
		Type: t,
		Data: utils.BytesToString(buf),
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		slog.Error("cannot persist the given clipboard data", "err", err)
		return
	}

	logdir := "./data/logs/clipboard"
	fpath := fmt.Sprintf("%s/%d/%d", logdir, date.Year(), date.Month())
	// The log is readable by the server's user alone: it is everything
	// copied, and it never belongs in the published store.
	err = os.MkdirAll(fpath, 0o700)
	if err != nil {
		slog.Error("cannot create the clipboard log folder", "path", fpath, "err", err)
		return
	}

	f, err := os.OpenFile(fmt.Sprintf("%s/%d.log", fpath, date.Day()),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		slog.Error("cannot open the clipboard log file", "path", fpath, "err", err)
		return
	}
	defer f.Close()

	all := utils.StringToBytes("---\n")
	all = append(all, data...)
	if _, err := f.Write(all); err != nil {
		slog.Error("cannot write the clipboard data to the log", "err", err)
		return
	}

}
