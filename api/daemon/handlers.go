// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
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

	// TODO: allocate urls using websocket action

	if in.SourcePath != "" {
		source = types.SourceAttachment
		b, err := ioutil.ReadFile(in.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %v, err: %w", in.SourcePath, err)
		}
		data = utils.BytesToString(b)

	}
	if in.DesiredPath != "" {
		// we want to make sure the extension of the file is correct
		dext := filepath.Ext(in.DesiredPath)
		sext := filepath.Ext(in.SourcePath)
		uri = strings.TrimSuffix(in.DesiredPath, dext) + sext
	}

	res, err := utils.Request(
		http.MethodPut,
		types.EndpointAllocateURL,
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

	url := config.Get().Domain + out.URL
	clipboard.Write(utils.StringToBytes(url))
	return &proto.AllocateURLOutput{URL: url, Message: "Done."}, nil
}

// CreateNews creates a news
func (m *Daemon) CreateNews(ctx context.Context, in *proto.CreateNewsInput) (out *proto.CreateNewsOutput, err error) {

	s := &types.ActionCreateNewsData{
		Date: in.Date, Title: in.Title, Body: in.Body,
	}
	b, _ := json.Marshal(s)

	m.writeCh <- &types.WebsocketMessage{
		Action:  types.ActionCreateNews,
		UserID:  m.ID,
		Message: "create news",
		Data:    b,
	}

	return &proto.CreateNewsOutput{Message: "DONE."}, nil
}

// CodeToImage tries to create an image for the given code.
func (m *Daemon) CodeToImage(ctx context.Context, in *proto.CodeToImageInput) (out *proto.CodeToImageOutput, err error) {
	log.Println("received a code2img request:", in.CodePath)
	var code string

	// the user presented a file, so we read it.
	// if it does not exist, then we don't bother the server.
	if len(in.CodePath) > 0 {
		b, err := ioutil.ReadFile(in.CodePath)
		if err != nil {
			return nil, fmt.Errorf("cannot read the given file: %w", err)
		}
		code = utils.BytesToString(b)
	}

	res, err := utils.Request(http.MethodPost, types.EndpointCode2Image, &types.Code2ImgInput{Code: code})
	if err != nil {
		return nil, fmt.Errorf("failed to convert: %w", err)
	}

	var o types.Code2ImgOutput
	err = json.Unmarshal(res, &o)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server response: %w", err)
	}

	// write to local clipboard.
	clipboard.Write(utils.StringToBytes(config.Get().Domain + o.Image))

	return &proto.CodeToImageOutput{
		CodeURL:  o.Code,
		ImageURL: o.Image,
	}, nil
}

// ListDaemons lists all active daemons.
func (m *Daemon) ListDaemons(ctx context.Context, in *proto.ListDaemonsInput) (out *proto.ListDaemonsOutput, err error) {
	readerCh := make(chan *types.WebsocketMessage)
	m.readChs.Store(m.ID, readerCh)
	m.writeCh <- &types.WebsocketMessage{
		Action:  types.ActionListDaemonsRequest,
		UserID:  m.ID,
		Message: "list active daemons",
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("list daemons timeout!")
			return nil, errors.New("list daemons timeout")
		case resp := <-readerCh:
			switch resp.Action {
			case types.ActionListDaemonsResponse:
				m.readChs.Delete(m.ID)
				return &proto.ListDaemonsOutput{Daemons: utils.BytesToString(resp.Data)}, nil
			default:
				log.Println(resp.Message)
			}
		}
	}
}
