// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package device

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"changkun.de/x/midgard/internal/types"
)

// TestNotOnList: a server that turns away a good sign-in says why, and the
// device tells that from any other failure, to say so rather than that it
// is offline.
func TestNotOnList(t *testing.T) {
	answer := func(code int, body string) *http.Response {
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}
	}
	for _, tt := range []struct {
		name string
		resp *http.Response
		want bool
	}{
		{"not on the list", answer(http.StatusForbidden, `{"msg":"`+types.MsgNotOnList+`"}`), true},
		{"blocked for failing", answer(http.StatusForbidden, ""), false},
		{"not signed in", answer(http.StatusUnauthorized, `{"msg":"`+types.MsgNotOnList+`"}`), false},
		{"no answer", nil, false},
	} {
		if got := notOnList(tt.resp); got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}
