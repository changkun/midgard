// Copyright 2021 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a GNU GPL-3.0 license that can be found in the LICENSE file.
//
// Written by Changkun Ou <changkun.de>

package code2img

import (
	"context"
	"fmt"
	"math"
	"net/url"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Render renders the given code string and returns a binary buffer
// that contains a carbon-now based image.
func Render(ctx context.Context, code string) ([]byte, error) {
	// https://carbon.now.sh/?
	// bg=rgba(74%2C144%2C226%2C1)&
	// t=material&
	// wt=none&
	// l=auto&
	// ds=true&
	// dsyoff=0px&
	// dsblur=29px&
	// wc=true&
	// wa=true&
	// pv=28px&
	// ph=100px&
	// ln=true&
	// fl=1&
	// fm=Source%20Code%20Pro&
	// fs=13.5px&
	// lh=152%25&
	// si=false&
	// es=2x&
	// wm=false&
	// code=func%2520main()
	var carbonOptions = map[string]string{
		"bg":     "rgba(74,144,226,1)", // backgroundColor
		"t":      "material",           // theme
		"wt":     "none",               // windowTheme
		"l":      "auto",               // language
		"ds":     "true",               // dropShadow
		"dsyoff": "0px",                // dropShadowOffsetY
		"dsblur": "29px",               // dropShadowBlurRadius
		"wc":     "true",               // windowControls
		"wa":     "true",               // widthAdjustment
		"pv":     "28px",               // paddingVertical
		"ph":     "100px",              // paddingHorizontal
		"ln":     "true",               // lineNumbers
		"fl":     "1",                  // firstLineNumber
		"fm":     "Source Code Pro",    // fontFamily
		"fs":     "13.5px",             // fontSize
		"lh":     "152%",               // lineHeight
		"si":     "false",              //squaredImage
		"es":     "2x",                 // exportSize
		"wm":     "false",              // watermark
	}

	values := url.Values{}
	for k, v := range carbonOptions {
		values.Set(k, v)
	}
	codeparam := url.Values{}
	codeparam.Set("code", url.PathEscape(code))

	ctx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	url := "https://carbon.now.sh/?" + values.Encode() + "&" + codeparam.Encode()

	var picbuf []byte
	sel := "#export-container  .container-bg"
	err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.EmulateViewport(2560, 1440),
		chromedp.Navigate(url),
		screenshot(sel, &picbuf, chromedp.NodeReady, chromedp.ByID),
	})
	if err != nil {
		return nil, fmt.Errorf("code2img: render task failed: %w", err)
	}
	return picbuf, nil
}

func screenshot(sel interface{}, picbuf *[]byte, opts ...chromedp.QueryOption) chromedp.QueryAction {
	if picbuf == nil {
		panic("picbuf cannot be nil")
	}

	return chromedp.QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		// get layout metrics
		_, _, contentSize, err := page.GetLayoutMetrics().Do(ctx)
		if err != nil {
			return err
		}

		width, height := int64(math.Ceil(contentSize.Width)), int64(math.Ceil(contentSize.Height))

		// force viewport emulation
		err = emulation.SetDeviceMetricsOverride(width, height, 1, false).
			WithScreenOrientation(&emulation.ScreenOrientation{
				Type:  emulation.OrientationTypePortraitPrimary,
				Angle: 0,
			}).
			Do(ctx)
		if err != nil {
			return err
		}

		// get box model
		box, err := dom.GetBoxModel().WithNodeID(nodes[0].NodeID).Do(ctx)
		if err != nil {
			return err
		}
		if len(box.Margin) != 8 {
			return chromedp.ErrInvalidBoxModel
		}

		// take screenshot of the box
		buf, err := page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatPng).
			WithClip(&page.Viewport{
				X:      math.Round(box.Margin[0]),
				Y:      math.Round(box.Margin[1]),
				Width:  math.Round(box.Margin[4] - box.Margin[0]),
				Height: math.Round(box.Margin[5] - box.Margin[1]),
				Scale:  1.0,
			}).Do(ctx)
		if err != nil {
			return err
		}

		*picbuf = buf
		return nil
	}, append(opts, chromedp.NodeVisible)...)
}
