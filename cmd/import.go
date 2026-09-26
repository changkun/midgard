// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"changkun.de/x/midgard/api/rest"
	"changkun.de/x/midgard/internal/config"
	"changkun.de/x/midgard/internal/store"
	"github.com/spf13/cobra"
)

var (
	importOwner string
	importFrom  string
	importDry   bool
)

// importCmd moves the shares an older server kept as files into the
// database, where they are served now. It runs on the server's machine, in
// the directory the server runs in, like mg server token.
var importCmd = &cobra.Command{
	Use:   "import --owner <owner> [--from ./data/repo] [--dry-run]",
	Short: "Import the shares an older server kept as files",
	Long: `Import the shares an older server kept as files, under ./data/repo, into
the database, where they are served now.

  mg server import --owner <owner> --dry-run   see what it would do
  mg server import --owner <owner>             do it

Every file becomes a share of the owner's, named by its path, so its old link
keeps working: data/repo/img/a.png is /midgard/img/a.png again. Hidden files,
the old git backup's .git among them, stay behind; they were never served. It
is safe to run again: a file already imported is skipped. The files are left
as they are; remove them once the links are checked.`,
	Args: cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		if importOwner == "" {
			errorf("--owner is required: the imported shares are someone's, to list and revoke")
			os.Exit(2)
		}
		s, err := store.Open(config.DBPath)
		if err != nil {
			errorf("cannot open the database: %v", err)
			os.Exit(1)
		}
		defer s.Close()

		r, err := importShares(context.Background(), s, importFrom, importOwner, importDry, os.Stderr)
		if err != nil {
			errorf("cannot import: %v", err)
			os.Exit(1)
		}
		verb := "imported"
		if importDry {
			verb = "would import"
		}
		errorf("%s %d, %d already imported, %d hidden left behind, %d failed.", verb, r.imported, r.present, r.hidden, r.failed)
		if r.failed > 0 {
			os.Exit(1)
		}
	},
}

func init() {
	importCmd.Flags().StringVar(&importOwner, "owner", "", "whose the shares become: the principal id, as for mg server token")
	importCmd.Flags().StringVar(&importFrom, "from", config.RepoPath, "the directory the older server kept its shares in")
	importCmd.Flags().BoolVarP(&importDry, "dry-run", "n", false, "report what would be imported, and change nothing")
	serverCmd.AddCommand(importCmd)
}

// importReport counts what importShares did: shares imported, files imported
// before, hidden files and folders left behind, and files it could not
// import, which it names as it goes.
type importReport struct{ imported, present, hidden, failed int }

// importShares makes a share of owner's of every file under dir, named by its
// path, and says on w what it could not import and why. With dry set, it
// only reports.
func importShares(ctx context.Context, s *store.Store, dir, owner string, dry bool, w io.Writer) (importReport, error) {
	var r importReport
	fail := func(name, why string) {
		fmt.Fprintf(w, "cannot import %s: %s\n", name, why)
		r.failed++
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if name == "." {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			// never served: the old git backup's .git, and dotfiles
			fmt.Fprintf(w, "left behind %s: hidden\n", name)
			r.hidden++
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			fail(name, "not a regular file") // a link may point anywhere
			return nil
		}
		name, err = rest.ShareName(name)
		if err != nil {
			fail(filepath.ToSlash(rel), err.Error())
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}

		switch sh, err := s.ShareByPath(ctx, name); {
		case err == nil && sh.Owner == owner && bytes.Equal(sh.Data, data):
			r.present++
			return nil
		case err == nil:
			fail(name, "the name is taken by another share")
			return nil
		case !errors.Is(err, store.ErrNotFound):
			return err
		}
		if dry {
			fmt.Fprintf(w, "would import %s\n", name)
			r.imported++
			return nil
		}
		_, err = s.ImportShare(ctx, owner, name, typeOf(name, data), data, fi.ModTime())
		if errors.Is(err, store.ErrTaken) {
			fail(name, "the name is taken by another share")
			return nil
		}
		if err != nil {
			return err
		}
		r.imported++
		return nil
	})
	return r, err
}

// typeOf is the type a file was served as: by its extension, or by what it
// holds, as the older server's file server did.
func typeOf(name string, data []byte) string {
	return cmp.Or(mime.TypeByExtension(path.Ext(name)), http.DetectContentType(data))
}
