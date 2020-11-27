// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package config

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
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
	Domain string  `yaml:"domain"`
	Server *Server `yaml:"server"`
	Daemon *Daemon `yaml:"daemon"`
}

// Server is the midgard server side configuration
type Server struct {
	Addr  string `yaml:"addr"`
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
	Addr   string `yaml:"addr"`
	Server string `yaml:"serfver"`
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
		_, filename, _, ok := runtime.Caller(1)
		if !ok {
			log.Fatalf("cannot get runtime caller")
		}
		p := path.Join(path.Dir(filename), "../../config.yml")
		d, err = ioutil.ReadFile(p)
		if err != nil {
			log.Fatalf("cannot read configuration, err: %v\n", err)
		}
	}
	err = yaml.Unmarshal(d, c)
	if err != nil {
		log.Fatalf("cannot parse configuration, err: %v\n", err)
	}
}
