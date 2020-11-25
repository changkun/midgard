// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package config

import (
	"io/ioutil"
	"log"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	conf *Config
	once sync.Once
)

// Config is a combination of all possible midgard configuration.
type Config struct {
	Title  string  `yaml:"title"`
	Server *Server `yaml:"server"`
	Daemon *Daemon `yaml:"daemon"`
}

// Server is the midgard server side configuration
type Server struct {
	HTTP  string `yaml:"http"`
	RPC   string `yaml:"rpc"`
	Mode  string `yaml:"mode"`
	Store struct {
		Prefix string `yaml:"prefix"`
		Path   string `yaml:"path"`
	} `yaml:"store"`
	Auth struct {
		User string `yaml:"user"`
		Pass string `yaml:"pass"`
	} `json:"auth"`
}

// Daemon is the midgard daemon configuration
type Daemon struct {
	ServerAddr string `yaml:"server_addr"`
}

// S returns the midgard server configuration
func S() *Server {
	load()
	return conf.Server
}

// D returns the midgard daemon configuration
func D() *Daemon {
	load()
	return conf.Daemon
}

// Get returns the whole midgard configuration
func Get() *Config {
	load()
	return conf
}

func load() {
	once.Do(func() {
		conf = &Config{}
		conf.parse()
	})
}

func (c *Config) parse() {
	f := os.Getenv("MIDGARD_CONF")
	d, err := ioutil.ReadFile(f)
	if err != nil {
		d, err = ioutil.ReadFile("./config.yml")
		if err != nil {
			log.Fatalf("cannot read configuration, err: %v\n", err)
		}
	}
	err = yaml.Unmarshal(d, c)
	if err != nil {
		log.Fatalf("cannot parse configuration, err: %v\n", err)
	}
}
