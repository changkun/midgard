// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rest

import (
	"log"
	"net/http"
	"path"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.design/x/midgard/pkg/config"
)

func (m *Midgard) routers() (r *gin.Engine) {
	gin.SetMode(config.S().Mode)

	r = gin.Default()
	r.NoRoute(staticHandler(config.S().Store.Prefix, config.S().Store.Path))
	r.LoadHTMLGlob(FixPath("./templates/*"))

	mg := r.Group("/midgard")
	mg.GET("/ping", m.PingPong)
	mg.GET("/news", m.News)

	v1 := mg.Group("/api/v1", BasicAuthWithAttemptsControl(Credentials{
		config.S().Auth.User: config.S().Auth.Pass,
	}))
	{
		v1.GET("/clipboard", m.GetFromUniversalClipboard)
		v1.POST("/clipboard", m.PutToUniversalClipboard)
		v1.GET("/ws", m.Subscribe)
		v1.PUT("/allocate", m.AllocateURL)
		v1.POST("/code2img", m.Code2img)
	}
	return
}

func staticHandler(prefix, root string) gin.HandlerFunc {
	fs := gin.Dir(root, false)
	fileServer := http.StripPrefix(prefix, http.FileServer(fs))

	return func(c *gin.Context) {
		file := strings.TrimPrefix(c.Request.URL.String(), prefix)
		// Check if file exists and/or if we have permission to access it
		f, err := fs.Open(file)
		if err != nil {
			c.Writer.WriteHeader(http.StatusNotFound)
			return
		}
		f.Close()
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}

// FixPath fixes a relative path
func FixPath(p string) string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		log.Fatalf("cannot get runtime caller")
	}
	return path.Join(path.Dir(filename), p)
}
