// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package config_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"golang.design/x/midgard/config"
)

func TestParseConfig(t *testing.T) {
	os.Setenv("MIDGARD_CONF", "../config.yml")
	t.Cleanup(func() {
		os.Setenv("MIDGARD_CONF", "")
	})
	conf := config.Get()
	fmt.Println(conf)

	// Test if all fields are filled.
	v := reflect.ValueOf(*conf)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.Struct {
			continue
		}
		if v.Field(i).Interface() != nil {
			continue
		}
		t.Fatalf("read empty from config, field: %v", v.Type().Field(i).Name)
	}
}
