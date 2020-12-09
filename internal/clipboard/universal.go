// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"sync"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"gopkg.in/yaml.v3"
)

var (
	// ErrTypeMismatch indicates an error that the existing data inside
	// the universal clipboard is inconsistent with the requested type.
	ErrTypeMismatch = errors.New("type mismatch")
)

type universalClipboard struct {
	typ types.ClipboardDataType
	buf []byte
	m   sync.Mutex
}

func (uc *universalClipboard) Read() (types.ClipboardDataType, []byte) {
	uc.m.Lock()
	defer uc.m.Unlock()
	buf := make([]byte, len(uc.buf))
	copy(buf, uc.buf)
	return uc.typ, buf
}

func (uc *universalClipboard) ReadAsImgage() (image.Image, error) {
	uc.m.Lock()
	defer uc.m.Unlock()

	if uc.typ != types.ClipboardDataTypeImagePNG {
		return nil, ErrTypeMismatch
	}

	return png.Decode(bytes.NewBuffer(uc.buf))
}

func (uc *universalClipboard) Get(t types.ClipboardDataType) []byte {
	uc.m.Lock()
	defer uc.m.Unlock()
	if t != uc.typ {
		return nil
	}

	buf := make([]byte, len(uc.buf))
	copy(buf, uc.buf)
	return buf
}

func (uc *universalClipboard) Put(t types.ClipboardDataType, buf []byte) bool {
	uc.m.Lock()
	defer uc.m.Unlock()
	if uc.typ == t && bytes.Compare(uc.buf, buf) == 0 {
		return false
	}

	uc.persist(t, buf)

	uc.typ = t
	uc.buf = buf
	return true
}

func (uc *universalClipboard) persist(t types.ClipboardDataType, buf []byte) {
	if t != types.ClipboardDataTypePlainText {
		buf = utils.StringToBytes(string(t))
	}

	date := time.Now().UTC()
	r := struct {
		Time time.Time
		Type types.ClipboardDataType
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
	err = os.MkdirAll(fpath, os.ModeDir|os.ModePerm)
	if err != nil {
		log.Println("cannot create log folder:", err)
		return
	}

	f, err := os.OpenFile(fmt.Sprintf("%s/%d.log", fpath, date.Day()),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
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

// Universal is the Midgard's universal clipboard.
//
// It holds a global shared storage that can be edited/fetched at anytime.
var Universal = universalClipboard{
	typ: types.ClipboardDataTypePlainText,
	buf: []byte{},
	m:   sync.Mutex{},
}
