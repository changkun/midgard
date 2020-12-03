package rest

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"changkun.de/x/midgard/pkg/clipboard"
	"changkun.de/x/midgard/pkg/code2img"
	"changkun.de/x/midgard/pkg/config"
	"changkun.de/x/midgard/pkg/types"
	"changkun.de/x/midgard/pkg/utils"
	"github.com/gin-gonic/gin"
)

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
		in.Code = utils.BytesToString(clipboard.Universal.Get(types.ClipboardDataTypePlainText))
	}

	// double check, if the code is still empty, then we don't want do anything
	if len(in.Code) == 0 {
		c.JSON(http.StatusBadRequest, &types.Code2ImgOutput{
			Message: "no code neither in your request or clipboard",
		})
		return
	}

	// save the code
	id := utils.NewUUID()
	codefile := "/code/" + id // no extension! we don't care which language is using.

	err := ioutil.WriteFile(config.S().Store.Path+codefile, utils.StringToBytes(in.Code), os.ModePerm)
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

	imgfile := "/img/" + id + ".png"
	err = ioutil.WriteFile(config.S().Store.Path+imgfile, imgb, os.ModePerm)
	if err != nil {
		c.JSON(http.StatusBadRequest, &types.Code2ImgOutput{
			Message: fmt.Sprintf("failed to save your image: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, &types.Code2ImgOutput{
		Code:    config.S().Store.Prefix + codefile,
		Image:   config.S().Store.Prefix + imgfile,
		Message: "Render success",
	})
	return
}
