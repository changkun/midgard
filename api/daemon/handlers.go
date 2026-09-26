// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/types/proto"
	"changkun.de/x/midgard/internal/utils"
	"changkun.de/x/midgard/internal/version"
)

// Ping response a pong
func (m *Daemon) Ping(ctx context.Context, in *proto.PingInput) (*proto.PingOutput, error) {
	return &proto.PingOutput{
		Version:   version.GitVersion,
		GoVersion: version.GoVersion,
		BuildTime: version.BuildTime,
	}, nil
}

// AllocateURL request the midgard server to allocate a given URL for
// a given resource, or the content from the midgard universal clipboard.
func (m *Daemon) AllocateURL(ctx context.Context, in *proto.AllocateURLInput) (*proto.AllocateURLOutput, error) {
	var (
		source = types.SourceUniversalClipboard
		data   string
		uri    string
	)

	// The mg command reads the file and sends its bytes. The daemon does not
	// open paths it is given: it answers any program on the machine, and
	// would publish any file it can read.
	if len(in.SourceData) > 0 {
		source = types.SourceAttachment
		data = base64.StdEncoding.EncodeToString(in.SourceData)
	}
	if in.DesiredPath != "" {
		// we want to make sure the extension of the file is correct
		dext := filepath.Ext(in.DesiredPath)
		sext := filepath.Ext(in.SourceName)
		uri = strings.TrimSuffix(in.DesiredPath, dext) + sext
	}

	res, err := utils.Request(
		http.MethodPut,
		types.EndpointAllocateURL(),
		&types.AllocateURLInput{
			Source: source,
			URI:    uri,
			Data:   data,
		})
	if err != nil {
		return nil, fmt.Errorf("cannot perform allocate request, err %w", err)
	}
	var out types.AllocateURLOutput
	err = json.Unmarshal(res, &out)
	if err != nil {
		return nil, fmt.Errorf("cannot parse requested URL, err: %w", err)
	}
	if out.URL == "" {
		return nil, fmt.Errorf("%s", out.Message)
	}

	url := config.ServerURL() + out.URL
	clipboard.Local.Write(types.MIMEPlainText, utils.StringToBytes(url))
	return &proto.AllocateURLOutput{URL: url, Message: "Done."}, nil
}

// CodeToImage tries to create an image for the given code.
func (m *Daemon) CodeToImage(ctx context.Context, in *proto.CodeToImageInput) (out *proto.CodeToImageOutput, err error) {
	slog.Info("received a code2img request", "bytes", len(in.Code))

	// An empty code asks the server to render the universal clipboard.
	res, err := utils.Request(http.MethodPost, types.EndpointCode2Image(), &types.Code2ImgInput{Code: in.Code})
	if err != nil {
		return nil, fmt.Errorf("failed to convert: %w", err)
	}

	var o types.Code2ImgOutput
	err = json.Unmarshal(res, &o)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server response: %w", err)
	}

	// write to local clipboard.
	clipboard.Local.Write(types.MIMEPlainText,
		utils.StringToBytes(config.ServerURL()+o.Image))

	return &proto.CodeToImageOutput{
		CodeURL:  o.Code,
		ImageURL: o.Image,
	}, nil
}

// ListDaemons lists all active daemons.
func (m *Daemon) ListDaemons(ctx context.Context, in *proto.ListDaemonsInput) (out *proto.ListDaemonsOutput, err error) {
	readerId, err := utils.NewUUIDShort()
	if err != nil {
		return nil, err
	}

	// The reader is always removed, and buffered: the connection hands every
	// message to every reader, and used to block on one whose request had
	// timed out, which stopped the daemon from syncing at all.
	readerCh := make(chan *types.WebsocketMessage, 8)
	m.readChs.Store(readerId, readerCh)
	defer m.readChs.Delete(readerId)

	select {
	case m.writeCh <- &types.WebsocketMessage{
		Action:  types.ActionListDaemonsRequest,
		UserID:  m.ID,
		Message: "list active daemons",
	}:
	case <-ctx.Done():
		return nil, errors.New("list daemons timeout: not connected to the server")
	}

	for {
		select {
		case <-ctx.Done():
			slog.Error("the list daemons request timed out")
			return nil, errors.New("list daemons timeout")
		case resp := <-readerCh:
			switch resp.Action {
			case types.ActionListDaemonsResponse:
				return &proto.ListDaemonsOutput{Daemons: utils.BytesToString(resp.Data)}, nil
			default:
				// not interested, ignore.
			}
		}
	}
}
