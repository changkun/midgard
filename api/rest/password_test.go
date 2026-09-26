// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"os"
	"strings"
	"testing"
)

func TestWeakPassword(t *testing.T) {
	for _, tt := range []struct {
		pass string
		weak bool
	}{
		{"", true},
		{examplePassword, true},
		{"a-real-password", false},
	} {
		if got := weakPassword(tt.pass); got != tt.weak {
			t.Errorf("weakPassword(%q) = %v, want %v", tt.pass, got, tt.weak)
		}
	}
}

// TestExamplePasswordMatchesConfigExample keeps the refusal honest: the
// placeholder it rejects must be the one config.example.yml ships with.
func TestExamplePasswordMatchesConfigExample(t *testing.T) {
	b, err := os.ReadFile("../../config.example.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "pass: "+examplePassword+"\n") {
		t.Fatalf("config.example.yml does not use the placeholder password %q", examplePassword)
	}
}
