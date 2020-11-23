// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package config

import (
	"io/ioutil"
	"log"
	"os"
	"runtime"

	"gopkg.in/yaml.v3"
)

// build info, assign by compile time or runtime.
var (
	Version   string
	BuildTime string
	GoVersion = runtime.Version()
)

var conf *Config

func init() {
	conf = &Config{}
	conf.parse()
}

// Config is a read-only midgard configuration center
type Config struct {
	Title string `yaml:"title"`
	Addr  struct {
		Host string `yaml:"host"`
		HTTP string `yaml:"http"`
		RPC  string `yaml:"rpc"`
	} `yaml:"addr"`
	Mode string `yaml:"mode"`
	Log  struct {
		Prefix string `yaml:"log"`
	} `yaml:"log"`
	Store struct {
		Prefix string `yaml:"prefix"`
		Path   string `yaml:"path"`
	} `yaml:"store"`
	Auth struct {
		User string `yaml:"user"`
		Pass string `yaml:"pass"`
	} `json:"auth"`
}

// Get returns the midgard configuration
func Get() *Config {
	return conf
}

func (c *Config) parse() {
	f := os.Getenv("MIDGARD_CONF")
	d, err := ioutil.ReadFile(f)
	if err != nil {
		d, err = ioutil.ReadFile("./server.yml")
		if err != nil {
			log.Fatalf("cannot read configuration, err: %v\n", err)
		}
	}
	err = yaml.Unmarshal(d, c)
	if err != nil {
		log.Fatalf("cannot parse configuration, err: %v\n", err)
	}
}
