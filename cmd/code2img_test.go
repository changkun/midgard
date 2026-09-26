// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCode(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	lf := write("lf.go", "one\ntwo\nthree\nfour\n")
	crlf := write("crlf.go", "one\r\ntwo\r\nthree\r\n")

	for _, tt := range []struct {
		name, path, lines, want string
	}{
		{"whole file", lf, "", "one\ntwo\nthree\nfour\n"},
		{"a range", lf, "2:3", "two\nthree"},
		{"one line", lf, "3:3", "three"},
		{"past the end", lf, "3:99", "three\nfour\n"},
		{"crlf range", crlf, "1:2", "one\ntwo"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readCode(tt.path, tt.lines)
			if err != nil || got != tt.want {
				t.Fatalf("readCode(%q) = %q, %v; want %q", tt.lines, got, err, tt.want)
			}
		})
	}

	for _, tt := range []struct{ name, path, lines string }{
		{"missing file", filepath.Join(dir, "missing.go"), ""},
		{"no colon", lf, "3"},
		{"not numbers", lf, "a:b"},
		{"zero start", lf, "0:2"},
		{"backwards", lf, "3:2"},
		// used to panic: the daemon trimmed a byte off an empty result
		{"start past the end", lf, "9:10"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := readCode(tt.path, tt.lines); err == nil {
				t.Fatalf("readCode(%q) = %q, want an error", tt.lines, got)
			}
		})
	}
}
