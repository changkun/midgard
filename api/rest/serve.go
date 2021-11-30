// Copyright 2020-2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package rest

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/office"
	"changkun.de/x/midgard/internal/utils"
)

// Midgard is the midgard server that serves all API endpoints.
type Midgard struct {
	s      *http.Server
	status *office.Status

	mu    sync.Mutex
	users *list.List
}

// NewMidgard creates a new midgard server
func NewMidgard() *Midgard {
	return &Midgard{status: office.NewStatus(), users: list.New()}
}

// Serve serves Midgard RESTful APIs.
func (m *Midgard) Serve() {
	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		q := make(chan os.Signal, 1)
		signal.Notify(q, os.Interrupt)
		sig := <-q
		log.Printf("%v", sig)
		cancel()

		log.Printf("shutting down api service ...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := m.s.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
			log.Printf("failed to shutdown api service: %v", err)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.refreshStatus(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.serveHTTP()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		backup(ctx)
	}()
	wg.Wait()

	log.Printf("api server is down, good bye!")
}

func (m *Midgard) serveHTTP() {
	addr := os.Getenv("MIDGARD_SERVER_ADDR")
	if len(addr) == 0 {
		addr = config.S().Addr
	}

	m.s = &http.Server{Handler: m.routers(), Addr: addr}
	log.Printf("server starting at http://%s", addr)
	err := m.s.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Printf("close with error: %v", err)
	}
}

func init() {
	if _, err := exec.LookPath("git"); err != nil {
		panic("please intall git on your system: sudo apt install git")
	}
}

// execute executes command inside the data folder.
func execute(dir, cmd string, args ...string) (out []byte, err error) {
	c := exec.Command(cmd, args...)
	c.Dir, err = filepath.Abs(dir)
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

	// initialize data as a git repo if not exists
	var old = "-old"
	var haveOld = false

	_, err := os.Stat(config.RepoPath)
	if !errors.Is(err, os.ErrNotExist) { // repo folder exists
		// mkdir data/repo-old
		log.Printf("mkdir %s", config.RepoPath+old)
		err = os.MkdirAll(config.RepoPath+old, fs.ModeDir|fs.ModePerm)
		if err != nil {
			log.Fatalf("cannot rename your folder: %v", err)
		}
		// cp -r data/repo data/repo-old
		log.Printf("cp -r %s %s", config.RepoPath, config.RepoPath+old)
		err = utils.Copy(config.RepoPath, config.RepoPath+old)
		if err != nil {
			log.Fatalf("cannot rename your folder: %v", err)
		}
		haveOld = true
		// rm -rf data/repo
		log.Printf("rm -rf %s", config.RepoPath)
		err = os.RemoveAll(config.RepoPath)
		if err != nil {
			log.Fatalf("cannot remove all your old files: %v", err)
		}
	}

	// git clone https://github.com/changkun/midgard-data repo
	log.Printf("git clone %s repo", config.S().Store.Backup.Repo)
	out, err := execute("./data", "git", "clone",
		config.S().Store.Backup.Repo, "repo")
	if err != nil {
		log.Println(utils.BytesToString(out))
		log.Fatalf("cannot clone your data repo: %v", err)
	}

	// move everything to the cloned folder
	// cp -r data/template data/repo
	repoTmpl := "./data/template"
	log.Printf("cp -r %s %s", repoTmpl, config.RepoPath)
	err = utils.Copy(repoTmpl, config.RepoPath)
	if err != nil {
		log.Fatalf("failed to merge old data into repo folder: %v", err)
	}
	if haveOld {
		// .git folder may fail to operate(permission denied),
		// set everything to 755.
		err := exec.Command("chmod", "-R", "0755", config.RepoPath).Run()
		if err != nil {
			log.Fatalf("failed to change permission: %v", err)
		}

		// cp -r data/repo-old data/repo
		log.Printf("cp -r %s %s", config.RepoPath+old, config.RepoPath)
		err = utils.Copy(config.RepoPath+old, config.RepoPath)
		if err != nil {
			log.Fatalf("failed to merge old data into repo folder: %v", err)
		}
	}

	// seems ok, start commit the local changes

	msg := fmt.Sprintf("midgard: backup %s", time.Now().Format(backupMsgTimeFmt))
	cmds := [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", msg},
		{"git", "push"},
	}
	for _, cc := range cmds {
		out, err = execute(config.RepoPath, cc[0], cc[1:]...)
		if err != nil {
			if strings.Contains(utils.BytesToString(out), "nothing to commit") ||
				strings.Contains(utils.BytesToString(out), "no changes added") {
				log.Println(utils.BytesToString(out))
				continue
			}
			log.Printf("cannot initialize your data folder: %v, details:", err)
			log.Fatalf("%s: %s\n", strings.Join(cc, " "), utils.BytesToString(out))
		}
	}

	err = os.RemoveAll(config.RepoPath + old)
	if err != nil {
		log.Fatalf("failed to remove your old data folder: %v", err)
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
				out, err = execute(config.RepoPath, cc[0], cc[1:]...)
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
				out, err = execute(config.RepoPath, cc[0], cc[1:]...)
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
