// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed by
// a GNU GPL-3.0 license that can be found in the LICENSE file.

package rest

import (
	"container/list"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.design/x/midgard/pkg/config"
	"golang.design/x/midgard/pkg/utils"
)

// Midgard is the midgard server that serves all API endpoints.
type Midgard struct {
	s *http.Server

	mu    sync.Mutex
	users *list.List
}

// NewMidgard creates a new midgard server
func NewMidgard() *Midgard {
	return &Midgard{users: list.New()}
}

// Serve serves Midgard RESTful APIs.
func (m *Midgard) Serve() {
	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		q := make(chan os.Signal, 1)
		signal.Notify(q, os.Interrupt, os.Kill)
		sig := <-q
		log.Printf("%v", sig)
		cancel()

		log.Printf("shutting down api service ...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := m.s.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
			log.Printf("failed to shudown api service: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		m.serveHTTP()
	}()
	go func() {
		defer wg.Done()
		backup(ctx)
	}()
	wg.Wait()

	log.Printf("api server is down, good bye!")
}

func (m *Midgard) serveHTTP() {
	m.s = &http.Server{Handler: m.routers(), Addr: config.S().Addr}
	log.Printf("server starting at http://%s", config.S().Addr)
	err := m.s.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Printf("close with error: %v", err)
	}
	return
}

func init() {
	if _, err := exec.LookPath("git"); err != nil {
		panic("please intall git on your system: sudo apt install git")
	}
}

func execute(cmd string, args ...string) (out []byte, err error) {
	c := exec.Command(cmd, args...)
	c.Dir, err = filepath.Abs(config.S().Store.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot check your data folder: %v", err)
	}

	out, err = c.CombinedOutput()
	return
}

// backup backups the data folder to a configured github repository
func backup(ctx context.Context) {
	// initialize data as a git repo if needed
	out, err := execute("git", "rev-parse", "--git-dir")
	if err != nil {
		log.Fatalf("cannot use git command from your system: %v", err)
	}
	if strings.Compare(utils.BytesToString(out), ".git\n") != 0 {
		// not a git repo, initialize it
		log.Println("out1:", string(out))
		log.Println("out2:", ".git", len(utils.BytesToString(out)), len(".git\n"))
		log.Println("use data folder for the first time, initialize it as a git repo...")

		cmds := [][]string{
			{"git", "init"},
			{"git", "add", "."},
			{"git", "commit", "-m", "initial commit"},
			{"git", "remote", "add", "origin", config.S().Store.Repo},
			{"git", "push", "-u", "origin", "master"},
		}
		for _, cc := range cmds {
			out, err = execute(cc[0], cc[1:]...)
			if err != nil {
				log.Fatalf("cannot initialize your data folder: %v, %s", err, utils.BytesToString(out))
			}
		}
		log.Println("initialize is finished, start regular backup...")
	} else {
		log.Println("backing up for the data repo..")
	}

	t := time.NewTicker(time.Duration(config.S().Store.Backup) * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now().Format("2006-01-02-15:04")
			// FIXME: very basic backup feature; should resolve conflict with remote?
			cmds := [][]string{
				{"git", "add", "."},
				{"git", "commit", "-m", fmt.Sprintf("backup at %s", now)},
				{"git", "push", "-u", "origin", "master"},
			}
			for _, cc := range cmds {
				out, err = execute(cc[0], cc[1:]...)
				if err != nil {
					if !strings.Contains(utils.BytesToString(out), "nothing to commit") {
						log.Printf("cannot backup your data: %v, %s", err, utils.BytesToString(out))
					} else {
						log.Println("nothing to backup.")
					}
					break
				}
			}
		}
	}
}
