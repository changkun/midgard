// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package config_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/testdata"
)

func TestMain(m *testing.M) {
	testdata.UseConfig()
	os.Exit(m.Run())
}

func TestParseConfig(t *testing.T) {
	conf := config.Get()
	fmt.Println(conf)

	// Test if all fields are filled.
	v := reflect.ValueOf(*conf)
	for i := range v.NumField() {
		if v.Field(i).Kind() == reflect.Struct {
			continue
		}
		if v.Field(i).Interface() != nil {
			continue
		}
		t.Fatalf("read empty from config, field: %v", v.Type().Field(i).Name)
	}
}
