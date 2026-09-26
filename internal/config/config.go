// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	conf *Config
	once sync.Once
)

// RepoPath is where an older server kept its shares, as files. They are
// served no longer; shares live in the database.
var RepoPath = "./data/repo"

// DBPath is the server's database (see internal/store): in a directory of
// its own in the data folder.
var DBPath = "./data/db/midgard.db"

// Config is a combination of all possible midgard configuration.
type Config struct {
	Title  string `yaml:"title"`
	Domain string `yaml:"domain"`
	// Token is an app token for this device, issued with mg server token
	// add, for a device that does not sign in with mg login.
	Token  string  `yaml:"token"`
	Server *Server `yaml:"server"`
}

// Server is the midgard server side configuration
type Server struct {
	Addr  string `yaml:"addr"`
	Mode  string `yaml:"mode"`
	Store struct {
		Prefix string `yaml:"prefix"`
	} `yaml:"store"`
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

// Load finds and reads the configuration, as the first Get does, but returns
// an error instead of ending the program: for one with no terminal to say it
// on, such as the Mac app, which asks for the server instead.
func Load() error {
	path, err := find()
	if err != nil {
		return err
	}
	c, err := read(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	once.Do(func() {})
	conf = c
	return nil
}

// Save sets the server this device uses, in the user's configuration
// directory, keeping the rest of what is there, and uses it from now on.
func Save(domain string) (path string, err error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path = filepath.Join(dir, "midgard", "config.yml")
	c, err := read(path)
	if err != nil {
		c = &Config{}
	}
	c.Domain = domain
	b, err := yaml.Marshal(c)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	// only its user may read it: it may hold an app token
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", err
	}
	once.Do(func() {})
	conf = c
	return path, nil
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

	// The source tree a binary was built in is no longer searched, as it
	// was for daemons installed before this lookup (#35): whatever
	// config.yml sat in a checkout was read by any program built there, the
	// Mac app and the tests included, and those daemons must sign in again
	// since the redesign anyway (specs/redesign.md §4).
	return "", fmt.Errorf("no config.yml in %s, and MIDGARD_CONF is not set: %w",
		strings.Join(candidates, ", "), fs.ErrNotExist)
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
