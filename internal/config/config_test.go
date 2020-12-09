// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package config_test

import (
	"fmt"
	"reflect"
	"testing"

	"changkun.de/x/midgard/internal/config"
)

func TestParseConfig(t *testing.T) {
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
