// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
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

// Universal is the Midgard's universal clipboard, it keeps the data in
// memory and logs its change history to the data store of midgard.
//
// It holds a global shared storage that can be edited/fetched at anytime.
var Universal UniversalClipboard = &universal{
	typ: types.MIMEPlainText,
	buf: []byte{},
}

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
	if uc.typ == t && bytes.Compare(uc.buf, buf) == 0 {
		return false
	}

	uc.log(t, buf)

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
		log.Println("cannot persist the given clipboard data:", err)
		return
	}

	logdir := config.S().Store.Path + "/logs/clipboard"
	fpath := fmt.Sprintf("%s/%d/%d", logdir, date.Year(), date.Month())
	err = os.MkdirAll(fpath, fs.ModeDir|fs.ModePerm)
	if err != nil {
		log.Println("cannot create log folder:", err)
		return
	}

	f, err := os.OpenFile(fmt.Sprintf("%s/%d.log", fpath, date.Day()),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, fs.ModePerm)
	if err != nil {
		log.Println("cannot open or create clipboard log file:", err)
		return
	}
	defer f.Close()

	all := utils.StringToBytes("---\n")
	all = append(all, data...)
	if _, err := f.Write(all); err != nil {
		log.Println("cannot write clipboard data to log:", err)
		return
	}

}
