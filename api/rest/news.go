// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rest

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/gin-gonic/gin"
	"golang.design/x/midgard/pkg/config"
)

type feeds struct {
	PageTitle string
	Items     []item

	IsErr      bool
	ErrMessage string
}

type item struct {
	Title string `yaml:"title"`
	Date  string `yaml:"date"`
	Body  string `yaml:"body"`
}

// News handles news page
//
// midgard news "title"
// > (Use Ctrl+D to complte)
// >
// > this is the content we want to share to the public.
// > support convert /midgard/fs/*.png pictures
// >
func (m *Midgard) News(c *gin.Context) {
	tmpl := "feeds.tmpl"

	f := feeds{PageTitle: "News", Items: []item{}}
	news := config.S().Store.Path + "/news"
	// TODO: order by date
	err := filepath.Walk(news, func(path string, info os.FileInfo, err error) error {
		// skip folders
		if info.IsDir() || err != nil {
			return nil
		}

		// skip non yml news
		if filepath.Ext(path) != ".yml" {
			return nil
		}

		// read content
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		var i item
		err = yaml.Unmarshal(b, &i)
		if err != nil {
			return fmt.Errorf("failed to parse news %s: %w", path, err)
		}
		f.Items = append(f.Items, i)
		return nil
	})
	if err != nil {
		f.IsErr = true
		f.ErrMessage = fmt.Sprintf("internal server error: %v", err)
		c.HTML(http.StatusInternalServerError, tmpl, f)
		return
	}

	c.HTML(http.StatusOK, tmpl, f)
}
