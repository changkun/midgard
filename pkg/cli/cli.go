// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"golang.design/x/midgard/config"
	"golang.design/x/midgard/pkg/clipboard"
	"golang.design/x/midgard/pkg/types"
	"golang.design/x/midgard/pkg/types/proto"
	"golang.design/x/midgard/pkg/utils"
)

var apiGen = "http://" + config.D().ServerAddr.HTTP + "/midgard/api/v1/allocate"

// AllocateURL request the midgard server to allocate a given URL for
// a given resource, or the content from the midgard universal clipboard.
func AllocateURL(dstpath, srcpath string) {
	var (
		source = types.SourceUniversalClipboard
		data   string
		uri    string
	)

	if srcpath != "" {
		source = types.SourceAttachment
		b, err := ioutil.ReadFile(srcpath)
		if err != nil {
			log.Fatalf("failed to read %v, err: %v", srcpath, err)
		}
		data = utils.BytesToString(b)

	}
	if dstpath != "" {
		// we want to make sure the extension of the file is correct
		dext := filepath.Ext(dstpath)
		sext := filepath.Ext(srcpath)
		uri = strings.TrimSuffix(dstpath, dext) + sext
	}

	res, err := utils.Request(http.MethodPut, apiGen, &types.AllocateURLInput{
		Source: source,
		URI:    uri,
		Data:   data,
	})
	var out types.AllocateURLOutput
	err = json.Unmarshal(res, &out)
	if err != nil {
		log.Fatalf("cannot parse requested URL, err: %v", err)
	}

	if out.URL != "" {
		url := config.D().ServerAddr.HTTP + out.URL
		clipboard.Write(utils.StringToBytes(url))
		fmt.Println("DONE: ", url)
	} else {
		fmt.Printf("%v\n", out.Message)
	}
}

// AllocateURLgRPC request the midgard server to allocate a given URL for
// a given resource, or the content from the midgard universal clipboard.
func AllocateURLgRPC(dstpath, srcpath string) {
	utils.Connect(func(ctx context.Context, c proto.MidgardClient) {
		var (
			source = proto.SourceType_UniversalClipboard
			data   string
			uri    string
		)

		if srcpath != "" {
			source = proto.SourceType_Attachment
			b, err := ioutil.ReadFile(srcpath)
			if err != nil {
				log.Fatalf("failed to read %v, err: %v", srcpath, err)
			}
			data = utils.BytesToString(b)
		}
		if dstpath != "" {
			// we want to make sure the extension of the file is correct
			dext := filepath.Ext(dstpath)
			sext := filepath.Ext(srcpath)
			uri = strings.TrimSuffix(dstpath, dext) + sext
		}

		out, err := c.AllocateURL(ctx, &proto.AllocateURLInput{
			Source: source,
			URI:    uri,
			Data:   data,
		})
		if err != nil {
			log.Fatalf("cannot requested RPC server, err: %v", err)
		}
		if out.URL != "" {
			url := config.D().ServerAddr.HTTP + out.URL
			clipboard.Write(utils.StringToBytes(url))
			fmt.Println("DONE: ", url)
		} else {
			fmt.Printf("%v\n", out.Message)
		}
	})
}
