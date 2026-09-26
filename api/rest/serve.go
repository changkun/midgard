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
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/utils"
)

// Midgard is the midgard server that serves all API endpoints.
type Midgard struct {
	s *http.Server

	mu    sync.Mutex
	users *list.List

	keepalive keepalive // how dead daemons are noticed
}

// NewMidgard creates a new midgard server
func NewMidgard() *Midgard {
	return &Midgard{users: list.New(), keepalive: defaultKeepalive}
}

// Serve serves Midgard RESTful APIs.
func (m *Midgard) Serve() {
	requirements()
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Go(func() {
		q := make(chan os.Signal, 1)
		signal.Notify(q, os.Interrupt)
		sig := <-q
		slog.Info("received a signal", "signal", sig)
		cancel()

		slog.Info("shutting down the api service")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := m.s.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("cannot shut down the api service", "err", err)
		}
	})
	wg.Go(func() {
		m.serveHTTP()
	})
	wg.Go(func() {
		backup(ctx)
	})
	wg.Wait()

	slog.Info("api server is down, good bye")
}

func (m *Midgard) serveHTTP() {
	addr := os.Getenv("MIDGARD_SERVER_ADDR")
	if len(addr) == 0 {
		addr = config.S().Addr
	}

	m.s = &http.Server{Handler: m.routers(), Addr: addr}
	slog.Info("api server is starting", "addr", "http://"+addr)
	err := m.s.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		slog.Error("api server closed with an error", "err", err)
	}
}

// examplePassword is the placeholder password in config.example.yml.
const examplePassword = "change-me"

// weakPassword reports whether p is a password nobody should serve with: none
// at all, or the published placeholder, which anyone can read.
func weakPassword(p string) bool { return p == "" || p == examplePassword }

// requirements checks what the server needs from the system it runs on. It
// runs when the server starts rather than at package init, because every mg
// command links this package and only the server needs these.
func requirements() {
	if weakPassword(config.S().Auth.Pass) {
		fatal("set server.auth.pass in config.yml: the server refuses to run " +
			"with an empty password or the one from config.example.yml")
	}
	if config.S().Store.Backup.Enable {
		if _, err := exec.LookPath("git"); err != nil {
			fatal("the backup feature needs git; install it or disable backup in config.yml")
		}
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
		slog.Info("the backup feature is disabled")
		return
	}

	// initialize data as a git repo if not exists
	var old = "-old"
	var haveOld = false

	_, err := os.Stat(config.RepoPath)
	if !errors.Is(err, os.ErrNotExist) { // repo folder exists
		// mkdir data/repo-old
		slog.Info("exec", "cmd", "mkdir "+config.RepoPath+old)
		err = os.MkdirAll(config.RepoPath+old, fs.ModeDir|fs.ModePerm)
		if err != nil {
			fatal("cannot rename your folder", "err", err)
		}
		// cp -r data/repo data/repo-old
		slog.Info("exec", "cmd", "cp -r "+config.RepoPath+" "+config.RepoPath+old)
		err = utils.Copy(config.RepoPath, config.RepoPath+old)
		if err != nil {
			fatal("cannot rename your folder", "err", err)
		}
		haveOld = true
		// rm -rf data/repo
		slog.Info("exec", "cmd", "rm -rf "+config.RepoPath)
		err = os.RemoveAll(config.RepoPath)
		if err != nil {
			fatal("cannot remove all your old files", "err", err)
		}
	}

	// git clone https://github.com/changkun/midgard-data repo
	slog.Info("exec", "cmd", "git clone "+config.S().Store.Backup.Repo+" repo")
	out, err := execute("./data", "git", "clone",
		config.S().Store.Backup.Repo, "repo")
	if err != nil {
		fatal("cannot clone your data repo",
			"err", err, "out", utils.BytesToString(out))
	}

	// move everything to the cloned folder
	// cp -r data/template data/repo
	repoTmpl := "./data/template"
	slog.Info("exec", "cmd", "cp -r "+repoTmpl+" "+config.RepoPath)
	err = utils.Copy(repoTmpl, config.RepoPath)
	if err != nil {
		fatal("cannot merge the old data into the repo folder", "err", err)
	}
	if haveOld {
		// .git folder may fail to operate(permission denied),
		// set everything to 755.
		err := exec.Command("chmod", "-R", "0755", config.RepoPath).Run()
		if err != nil {
			fatal("cannot change the repo folder permission", "err", err)
		}

		// cp -r data/repo-old data/repo
		slog.Info("exec", "cmd", "cp -r "+config.RepoPath+old+" "+config.RepoPath)
		err = utils.Copy(config.RepoPath+old, config.RepoPath)
		if err != nil {
			fatal("cannot merge the old data into the repo folder", "err", err)
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
				slog.Info("nothing to back up", "out", utils.BytesToString(out))
				continue
			}
			fatal("cannot initialize your data folder", "err", err,
				"cmd", strings.Join(cc, " "), "out", utils.BytesToString(out))
		}
	}

	err = os.RemoveAll(config.RepoPath + old)
	if err != nil {
		fatal("cannot remove your old data folder", "err", err)
	}
	slog.Info("the backup feature is enabled")

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
					slog.Error("cannot resolve the backup conflict", "err", err,
						"cmd", strings.Join(cc, " "), "out", utils.BytesToString(out))
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
					slog.Error("cannot back up your data", "err", err,
						"cmd", strings.Join(cc, " "), "out", utils.BytesToString(out))
					// FIXME: email notification: ask manual action (very rare?)
					goto start
				}
			}
			slog.Info(msg)
		}
	}
}

// fatal logs at the error level and exits. log/slog has no Fatal, and the
// backup bootstrap must not continue past a failed step.
func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
