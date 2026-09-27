// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

// Package client is how mg commands talk to the midgard server: directly,
// over HTTP, with the device's credentials. They used to go through the local
// daemon over gRPC, which only relayed each call to the server.
package client

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/e2e"
	"changkun.de/x/midgard/internal/signin"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"changkun.de/x/midgard/internal/wire"
)

// Share publishes data at a link and returns it. To share the clipboard, a
// client reads it first (Clipboard) and sends its bytes: the server does not
// ask a device for them (specs/redesign.md §11). A name, when given, is a link of its own
// besides the random one. filename, the source file's name if there is one,
// gives the share its type and the name its extension. A positive expires
// retires the share after that long.
func Share(name string, data []byte, filename string, expires time.Duration) (types.ShareInfo, error) {
	in := types.ShareInput{ExpiresIn: int64(expires / time.Second)}
	if len(data) > 0 {
		in.Data = base64.StdEncoding.EncodeToString(data)
		in.Type = types.MIME(cmp.Or(mime.TypeByExtension(path.Ext(filename)), http.DetectContentType(data)))
	}
	in.Name = name
	if ext := path.Ext(filename); name != "" && ext != "" {
		// the link takes the file's extension, so it opens as what it is
		in.Name = strings.TrimSuffix(name, path.Ext(name)) + ext
	}
	var out types.ShareInfo
	if err := call(http.MethodPost, types.EndpointShares(), &in, &out); err != nil {
		return out, err
	}
	out.URL = config.ServerURL() + out.URL
	return out, nil
}

// Shares lists this person's shares, newest first, with full links.
func Shares() ([]types.ShareInfo, error) {
	var out types.SharesOutput
	if err := call(http.MethodGet, types.EndpointShares(), nil, &out); err != nil {
		return nil, err
	}
	for i := range out.Shares {
		out.Shares[i].URL = config.ServerURL() + out.Shares[i].URL
	}
	return out.Shares, nil
}

// DeleteShare revokes this person's share slug; its links stop working.
func DeleteShare(slug string) error {
	return call(http.MethodDelete, types.EndpointShares()+"/"+slug, nil, nil)
}

// Devices lists this person's devices, online or not.
func Devices() ([]types.Device, error) {
	var out types.DevicesOutput
	err := call(http.MethodGet, types.EndpointDevices(), nil, &out)
	return out.Devices, err
}

// ForgetDevice forgets this person's device id: the server stops holding
// copies for it, until it connects again.
func ForgetDevice(id string) error {
	return call(http.MethodDelete, types.EndpointDevices()+"/"+id, nil, nil)
}

// Queue is what the server holds until each of this person's devices has
// it.
func Queue() (types.QueueOutput, error) {
	var out types.QueueOutput
	err := call(http.MethodGet, types.EndpointQueue(), nil, &out)
	return out, err
}

// TakeBack takes back a copy none of this person's devices has yet.
func TakeBack(seq uint64) error {
	return call(http.MethodDelete, types.EndpointQueue()+"/"+strconv.FormatUint(seq, 10), nil, nil)
}

// ErrNotFound means the server has no such thing for this person.
var ErrNotFound = errors.New("not found")

// ErrNotPaired means this person's copies are sealed with a key this device
// does not have (specs/redesign.md §11): it must pair with one that has it.
var ErrNotPaired = errors.New("this device does not have your key: pair it with one of your devices")

// statusError is an answer the server refused with.
type statusError struct {
	code int
	msg  string
}

func (e *statusError) Error() string { return fmt.Sprintf("the server answered %d: %s", e.code, e.msg) }

// ErrNoDevice means none of this person's devices is online to answer a read
// of their clipboard or history, which are on the devices.
var ErrNoDevice = errors.New("none of your devices is online")

// call makes a request and turns a failure into an error, with the server's
// reason when it gives one.
func call(method, api string, in, out any) error {
	status, body, err := utils.Do(method, api, in)
	if errors.Is(err, signin.ErrSignedOut) {
		return err
	}
	if err != nil {
		return fmt.Errorf("cannot reach the midgard server: %w", err)
	}
	switch {
	case status == http.StatusNotFound:
		return ErrNotFound
	case status == http.StatusServiceUnavailable:
		return ErrNoDevice
	case status >= 300:
		var reason struct {
			Msg string `json:"msg"`
		}
		json.Unmarshal(body, &reason)
		return &statusError{status, cmp.Or(reason.Msg, http.StatusText(status))}
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}

// History lists this person's clipboard history, newest first, each text
// with its start, opened with key when sealed.
func History(key *e2e.Key) ([]types.HistoryEntry, error) {
	var out types.HistoryOutput
	if err := call(http.MethodGet, types.EndpointHistory(), nil, &out); err != nil {
		return nil, err
	}
	for i, e := range out.History {
		if e.Kid == "" {
			continue
		}
		if key == nil || key.ID() != e.Kid {
			return nil, ErrNotPaired
		}
		sealed, err := base64.StdEncoding.DecodeString(e.Preview)
		if err != nil {
			return nil, err
		}
		preview, err := key.Open(e2e.Preview, []wire.Format{{MIME: string(e.Type), Size: e.Size}}, sealed)
		if err != nil {
			return nil, err
		}
		out.History[i].Preview, out.History[i].Kid = validPrefix(preview), ""
	}
	return out.History, nil
}

// validPrefix is b as text, without a character a preview cut in two.
func validPrefix(b []byte) string {
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return string(b)
}

// HistoryEntry returns the copy numbered id from this person's history.
func HistoryEntry(key *e2e.Key, id int64) (types.MIME, []byte, error) {
	var out types.ClipboardData
	if err := call(http.MethodGet, fmt.Sprintf("%s/%d", types.EndpointHistory(), id), nil, &out); err != nil {
		return "", nil, err
	}
	return open(key, out)
}

// DeleteHistoryEntry removes the copy numbered id from this person's history.
func DeleteHistoryEntry(id int64) error {
	return call(http.MethodDelete, fmt.Sprintf("%s/%d", types.EndpointHistory(), id), nil, nil)
}

// ClearHistory removes all of this person's history.
func ClearHistory() error {
	return call(http.MethodDelete, types.EndpointHistory(), nil, nil)
}

// Copy puts data on this person's clipboard through the server, which hands
// it to their devices, and returns its number in the history. A command
// cannot keep it there itself: on X11 and Wayland a copy lasts only while the
// program that made it runs, and a command exits at once. The daemons run on.
func Copy(key *e2e.Key, t types.MIME, data []byte) (uint64, error) {
	in := types.PutToUniversalClipboardInput{ClipboardData: types.ClipboardData{Type: t, Data: string(data)}}
	switch {
	case key != nil:
		sealed, err := key.Seal(e2e.Copy, []wire.Format{{MIME: string(t), Size: len(data)}}, data)
		if err != nil {
			return 0, err
		}
		in.Data, in.Kid = base64.StdEncoding.EncodeToString(sealed), key.ID()
	case t == types.MIMEImagePNG:
		in.Data = base64.StdEncoding.EncodeToString(data)
	}
	var out types.PutToUniversalClipboardOutput
	err := call(http.MethodPost, types.EndpointClipboard(), &in, &out)
	var se *statusError
	if key == nil && errors.As(err, &se) && se.code == http.StatusConflict {
		return 0, ErrNotPaired // its person's copies are sealed, and it has no key
	}
	return out.Seq, err
}

// Clipboard is this person's clipboard: the newest copy, from one of their
// devices or from what the server holds, opened with key when sealed.
// ErrNoDevice when there is neither.
func Clipboard(key *e2e.Key) (types.MIME, []byte, error) {
	var out types.ClipboardData
	if err := call(http.MethodGet, types.EndpointClipboard(), nil, &out); err != nil {
		return "", nil, err
	}
	return open(key, out)
}

// open is the bytes of a copy the server answered with: sealed, opened with
// key, which must be the one it names.
func open(key *e2e.Key, d types.ClipboardData) (types.MIME, []byte, error) {
	if d.Kid == "" {
		return decode(d)
	}
	if key == nil || key.ID() != d.Kid {
		return "", nil, ErrNotPaired
	}
	sealed, err := base64.StdEncoding.DecodeString(d.Data)
	if err != nil || len(sealed) < e2e.Overhead {
		return "", nil, errors.New("the server answered with a copy that is not one")
	}
	plain, err := key.Open(e2e.Copy, []wire.Format{{MIME: string(d.Type), Size: len(sealed) - e2e.Overhead}}, sealed)
	return d.Type, plain, err
}

// decode is the bytes of a copy as the API encodes it: an image in base64.
func decode(d types.ClipboardData) (types.MIME, []byte, error) {
	if d.Type == types.MIMEImagePNG {
		b, err := base64.StdEncoding.DecodeString(d.Data)
		return d.Type, b, err
	}
	return d.Type, []byte(d.Data), nil
}

// Kid is the id of this person's key, "" while they have none
// (specs/redesign.md §11).
func Kid() (string, error) {
	var out types.KeyOutput
	err := call(http.MethodGet, types.EndpointKey(), nil, &out)
	return out.Kid, err
}

// LeavePairing leaves a pairing box in its mailbox, for a new device.
func LeavePairing(mailbox string, box []byte) error {
	return call(http.MethodPost, types.EndpointPair(), types.PairInput{
		Mailbox: mailbox, Box: base64.StdEncoding.EncodeToString(box),
	}, nil)
}

// PairingWaits reports whether the pairing box left in mailbox still waits
// for its new device, without taking it.
// A server from before this answers 404, an error: it cannot say.
func PairingWaits(mailbox string) (bool, error) {
	err := call(http.MethodHead, types.EndpointPair()+"/"+mailbox, nil, nil)
	var gone *statusError
	if errors.As(err, &gone) && gone.code == http.StatusGone {
		return false, nil
	}
	return err == nil, err
}

// TakePairing takes the pairing box waiting in its mailbox; ErrNotFound when
// none does, as it was used or its ten minutes passed.
func TakePairing(mailbox string) ([]byte, error) {
	var out types.PairOutput
	if err := call(http.MethodGet, types.EndpointPair()+"/"+mailbox, nil, &out); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(out.Box)
}
