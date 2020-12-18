// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package config

import (
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
		Backup struct {
			Enable   bool   `yaml:"enable"`
			Interval int    `yaml:"interval"`
			Repo     string `yaml:"repo"`
		} `yaml:"backup"`
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
	d, err := os.ReadFile(f)
	if err != nil {
		fix := func(p string) string { // fixes a relative path
			_, filename, _, ok := runtime.Caller(1)
			if !ok {
				log.Fatalf("cannot get runtime caller")
			}
			return path.Join(path.Dir(filename), p)
		}

		p := fix("../../config.yml")
		d, err = os.ReadFile(p)
		if err != nil {
			log.Fatalf("cannot read configuration, err: %v\n", err)
		}
	}
	err = yaml.Unmarshal(d, c)
	if err != nil {
		log.Fatalf("cannot parse configuration, err: %v\n", err)
	}
}
