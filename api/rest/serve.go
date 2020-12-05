// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

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

	"changkun.de/x/midgard/pkg/config"
	"changkun.de/x/midgard/pkg/utils"
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

const backupMsgTimeFmt = "2006-01-02 15:04"

// backup backups the data folder to a configured github repository
func backup(ctx context.Context) {
	if !config.S().Store.Backup.Enable {
		log.Println("backup feature is disabled.")
		return
	}

	// initialize data as a git repo if needed
	ext := "-old"
	out, err := execute("git", "rev-parse", "--git-dir")
	if err != nil {
		log.Fatalf("cannot use git command from your system: %v", err)
	}
	if strings.Compare(utils.BytesToString(out), ".git\n") != 0 {
		// not a git repo, rename it as old
		err := os.Rename(config.S().Store.Path, config.S().Store.Path+ext)
		if err != nil {
			log.Fatalf("cannot rename your folder: %v", err)
		}
		err = os.Mkdir(config.S().Store.Path, os.ModeDir|os.ModePerm)
		if err != nil {
			log.Fatalf("cannot create a new data folder: %v", err)
		}
		// clone the remote repo
		cmds := [][]string{
			{"git", "clone", config.S().Store.Backup.Repo, "."},
		}
		for _, cc := range cmds {
			out, err = execute(cc[0], cc[1:]...)
			if err != nil {
				log.Fatalf("cannot clone your data folder: %v", err)
			}
		}

		// move everything to the cloned folder
		err = utils.Copy(config.S().Store.Path+ext, config.S().Store.Path)
		if err != nil {
			log.Fatalf("failed to merge your old local data folder into your remote data folder: %v", err)
		}

		// seems ok, start commit the local changes

		msg := fmt.Sprintf("midgard: backup %s", time.Now().Format(backupMsgTimeFmt))
		cmds = [][]string{
			{"git", "add", "."},
			{"git", "commit", "-m", msg},
			{"git", "push"},
		}
		for _, cc := range cmds {
			out, err = execute(cc[0], cc[1:]...)
			if err != nil {
				if strings.Contains(utils.BytesToString(out), "nothing to commit") {
					continue
				}
				log.Printf("cannot initialize your data folder: %v, details:", err)
				log.Fatalf("%s: %s\n", strings.Join(cc, " "), utils.BytesToString(out))
			}
		}

		err = os.RemoveAll(config.S().Store.Path + ext)
		if err != nil {
			log.Fatalf("failed to remove your old data folder: %v", err)
		}
	}
	log.Println("backup is enabled.")

	t := time.NewTicker(time.Duration(config.S().Store.Backup.Interval) * time.Minute)
	for {
	start:
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// basic conflict resolve, are there any other failures?
			cmds := [][]string{
				{"git", "stash"},
				{"git", "fetch"},
				{"git", "rebase"},
				{"git", "stash", "pop"},
			}
			for _, cc := range cmds {
				out, err = execute(cc[0], cc[1:]...)
				if err != nil {
					if strings.Contains(utils.BytesToString(out), "No stash entries") {
						continue
					}
					log.Printf("failed to resolve conflict: %v, details:", err)
					log.Printf("%s: %s\n", strings.Join(cc, " "), utils.BytesToString(out))
					// FIXME: email notification: ask manual action (very rare?)
					goto start
				}
			}

			// add, commit, and push
			msg := fmt.Sprintf("midgard: backup at %s", time.Now().Format(backupMsgTimeFmt))
			cmds = [][]string{
				{"git", "add", "."},
				{"git", "commit", "-m", msg},
				{"git", "push"},
			}
			for _, cc := range cmds {
				out, err = execute(cc[0], cc[1:]...)
				if err != nil {
					if strings.Contains(utils.BytesToString(out), "nothing to commit") {
						continue
					}
					log.Printf("cannot backup your data: %v, details:\n", err)
					log.Printf("%s: %s\n", strings.Join(cc, " "), utils.BytesToString(out))
					// FIXME: email notification: ask manual action (very rare?)
					goto start
				}
			}
			log.Println(msg)
		}
	}
}
