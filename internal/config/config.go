// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package config

import (
	"encoding/base64"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	conf *Config
	once sync.Once
)

// RepoPath points to the actual storage
var RepoPath = "./data/repo"

// DBPath is the server's database (see internal/store): in a directory of
// its own in the data folder, outside RepoPath, which is published.
var DBPath = "./data/db/midgard.db"

// Config is a combination of all possible midgard configuration.
type Config struct {
	Title  string `yaml:"title"`
	Domain string `yaml:"domain"`
	// Token is this device's token, issued with mg server token add. A
	// device with a token needs no server.auth credentials.
	Token  string  `yaml:"token"`
	Server *Server `yaml:"server"`
}

// Server is the midgard server side configuration
type Server struct {
	Addr  string `yaml:"addr"`
	Mode  string `yaml:"mode"`
	Store struct {
		Prefix string `yaml:"prefix"`
		// LogClipboard keeps every text copied on any device, in plain
		// text, under data/logs/clipboard. It is off unless asked for:
		// what people copy includes passwords.
		LogClipboard bool `yaml:"log_clipboard"`
	} `yaml:"store"`
	Auth struct {
		User string `yaml:"user"`
		Pass string `yaml:"pass"`
	} `json:"auth"`
	// TrustedProxies lists the networks whose X-Forwarded-For header is
	// believed when working out a client's address, which the login
	// attempt limit is keyed by. Empty means loopback and private networks,
	// where a reverse proxy in front of midgard usually sits.
	TrustedProxies []string `yaml:"trusted_proxies"`
}

// S returns the midgard server configuration
func S() *Server {
	load()
	return conf.Server
}

// Get returns the whole midgard configuration
func Get() *Config {
	load()
	return conf
}

// ServerURL is the midgard server's base URL, such as https://example.com or
// http://mg.local:8456, with no trailing slash. It is the domain setting, which
// may carry its scheme and port. Without a scheme, http is assumed for
// localhost and 0.0.0.0 and https otherwise, which is how the domain was
// always read; a server on plain http elsewhere, such as a Raspberry Pi on the
// home network, needs the scheme spelled out.
func ServerURL() string {
	d := strings.TrimRight(Get().Domain, "/")
	switch {
	case strings.HasPrefix(d, "http://"), strings.HasPrefix(d, "https://"):
		return d
	case strings.Contains(d, "localhost"), strings.Contains(d, "0.0.0.0"):
		return "http://" + d
	default:
		return "https://" + d
	}
}

// Authorization is the Authorization header a client sends to the server:
// this device's token when it has one, and the server's credentials
// otherwise, as every device needed before tokens existed.
func Authorization() string {
	if t := Get().Token; t != "" {
		return "Bearer " + t
	}
	var user, pass string
	if s := Get().Server; s != nil {
		user, pass = s.Auth.User, s.Auth.Pass
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

// load reads the configuration the first time it is asked for, not when the
// program starts, so a command that needs none — mg version, mg help — runs
// without one.
func load() {
	once.Do(func() {
		path, err := find()
		if err != nil {
			slog.Error("cannot find the configuration", "err", err)
			os.Exit(1)
		}
		conf, err = read(path)
		if err != nil {
			slog.Error("cannot read the configuration", "path", path, "err", err)
			os.Exit(1)
		}
	})
}

// find returns the path of the configuration file. It looks, in order, at:
//
//  1. $MIDGARD_CONF, when set;
//  2. config.yml in the working directory;
//  3. midgard/config.yml in the user's configuration directory, such as
//     ~/.config on Linux and ~/Library/Application Support on macOS.
//
// A MIDGARD_CONF that names a missing file is an error rather than a reason to
// look elsewhere: whoever set it meant that file.
func find() (string, error) {
	if p := os.Getenv("MIDGARD_CONF"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("MIDGARD_CONF=%s: %w", p, err)
		}
		return p, nil
	}

	candidates := []string{"config.yml"}
	if dir, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(dir, "midgard", "config.yml"))
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// Before the lookup above existed, the configuration was read from the
	// source tree the binary was built in. Installed daemons still depend on
	// that, so keep finding it, but say where it should move to.
	if p := sourceTreeConfig(); p != "" {
		if _, err := os.Stat(p); err == nil {
			slog.Warn("reading the configuration from the source tree; "+
				"move it to one of the searched locations",
				"path", p, "searched", strings.Join(candidates, ", "))
			return p, nil
		}
	}
	return "", fmt.Errorf("no config.yml in %s, and MIDGARD_CONF is not set: %w",
		strings.Join(candidates, ", "), fs.ErrNotExist)
}

// sourceTreeConfig is where config.yml sat relative to this file's source,
// which only exists on the machine that built the binary.
func sourceTreeConfig() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "config.yml")
}

// read parses the configuration file at path.
func read(path string) (*Config, error) {
	d, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Config{}
	if err := yaml.Unmarshal(d, c); err != nil {
		return nil, err
	}
	return c, nil
}
