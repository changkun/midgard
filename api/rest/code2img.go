// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"time"

	"changkun.de/x/midgard/internal/clipboard"
	"changkun.de/x/midgard/internal/code2img"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/types"
	"changkun.de/x/midgard/internal/utils"
	"github.com/gin-gonic/gin"
)

func init() {
	// tries to find the Chrome browser somewhere in the current system.
	// It performs a rather agressive search, which is the same in all
	// systems. That may make it a bit slow, but it will only be run at
	// boot time.
	for _, path := range [...]string{
		// Unix-like
		"headless_shell",
		"headless-shell",
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		"google-chrome-beta",
		"google-chrome-unstable",
		"/usr/bin/google-chrome",

		// Windows
		"chrome",
		"chrome.exe", // in case PATHEXT is misconfigured
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,

		// Mac
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	} {
		_, err := exec.LookPath(path)
		if err == nil {
			return
		}
	}
	panic("please intall google-chrome on your system (required by code2img).")
}

const code2imgTimeFormat = "060102-150405"

// Code2img code to image handler
func (m *Midgard) Code2img(c *gin.Context) {
	var in types.Code2ImgInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, &types.Code2ImgOutput{
			Message: "bad inputs",
		})
		return
	}

	// if the request does not send any code, then let's use the data
	// from our universal clipboard
	if len(in.Code) == 0 {
		in.Code = utils.BytesToString(clipboard.Universal.ReadAs(types.MIMEPlainText))
	}

	// double check, if the code is still empty, then we don't want do anything
	if len(in.Code) == 0 {
		c.JSON(http.StatusBadRequest, &types.Code2ImgOutput{
			Message: "no code neither in your request or clipboard",
		})
		return
	}

	// save the code
	id := time.Now().UTC().Format(code2imgTimeFormat)
	codefile := "/code/" + id // no extension! we don't care which language is using.

	err := os.WriteFile(config.RepoPath+codefile, utils.StringToBytes(in.Code), fs.ModePerm)
	if err != nil {
		c.JSON(http.StatusBadRequest, &types.Code2ImgOutput{
			Message: fmt.Sprintf("failed to save your code: %v", err),
		})
		return
	}

	// render and save the image
	imgb, err := code2img.Render(in.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, &types.Code2ImgOutput{
			Message: fmt.Sprintf("failed to render code image: %v", err),
		})
		return
	}

	imgfile := "/code/" + id + ".png"
	err = os.WriteFile(config.RepoPath+imgfile, imgb, fs.ModePerm)
	if err != nil {
		c.JSON(http.StatusBadRequest, &types.Code2ImgOutput{
			Message: fmt.Sprintf("failed to save your image: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, &types.Code2ImgOutput{
		Code:    config.S().Store.Prefix + codefile,
		Image:   config.S().Store.Prefix + imgfile,
		Message: "render success",
	})
	return
}
