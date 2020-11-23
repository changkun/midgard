// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"golang.design/x/midgard/clipboard"
	"golang.design/x/midgard/config"
	"golang.design/x/midgard/types"
	"golang.design/x/midgard/utils"
)

func requestURI() {
	var (
		apiGen = "http://" + config.Get().Addr.HTTP + "/midgard/api/v1/generate"
		source = types.SourceUniversalClipboard
		data   string
		uri    = *genpath
	)

	if *fromfile != "" {
		source = types.SourceAttachment
		b, err := ioutil.ReadFile(*fromfile)
		if err != nil {
			log.Fatalf("failed to read your file %v, err: %v", *fromfile, err)
		}
		data = string(b)

		if *genpath != "" {
			// we want to make sure the extension of the file is correct
			uext := filepath.Ext(*genpath)
			fext := filepath.Ext(*fromfile)
			uri = strings.TrimSuffix(*genpath, uext) + fext
		}
	}

	res, err := utils.Request(http.MethodPut, apiGen, &types.URIGeneratorInput{
		Source: source,
		URI:    uri,
		Data:   data,
	})
	var out types.URIGeneratorOutput
	err = json.Unmarshal([]byte(res), &out)
	if err != nil {
		log.Fatalf("cannot parse requested URL, err: %v", err)
	}

	url := config.Get().Addr.Host + config.Get().Addr.HTTP + out.URL
	clipboard.Write([]byte(url))
	fmt.Println("DONE: ", url)
}
