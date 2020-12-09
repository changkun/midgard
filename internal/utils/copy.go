// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package utils

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// this implemnetation is derived from https://github.com/otiai10/copy.

const (
	// tmpPermissionForDirectory makes the destination directory writable,
	// so that stuff can be copied recursively even if any original directory is NOT writable.
	// See https://github.com/otiai10/copy/pull/9 for more information.
	tmpPermissionForDirectory = os.FileMode(0755)
)

// Copy copies src to dest, doesn't matter if src is a directory or a file.
func Copy(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	return switchboard(src, dest, info)
}

// switchboard switches proper copy functions regarding file type, etc...
// If there would be anything else here, add a case to this switchboard.
func switchboard(src, dest string, info os.FileInfo) error {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return onsymlink(src, dest, info)
	case info.IsDir():
		return dcopy(src, dest, info)
	default:
		return fcopy(src, dest, info)
	}
}

// fcopy is for just a file,
// with considering existence of parent directory
// and file permission.
func fcopy(src, dest string, info os.FileInfo) (err error) {

	if err = os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return
	}

	f, err := os.Create(dest)
	if err != nil {
		return
	}
	defer fclose(f, &err)

	if err = os.Chmod(f.Name(), info.Mode()); err != nil {
		return
	}

	s, err := os.Open(src)
	if err != nil {
		return
	}
	defer fclose(s, &err)

	if _, err = io.Copy(f, s); err != nil {
		return
	}

	return
}

// dcopy is for a directory,
// with scanning contents inside the directory
// and pass everything to "copy" recursively.
func dcopy(srcdir, destdir string, info os.FileInfo) (err error) {

	originalMode := info.Mode()

	// Make dest dir with 0755 so that everything writable.
	if err = os.MkdirAll(destdir, tmpPermissionForDirectory); err != nil {
		return
	}
	// Recover dir mode with original one.
	defer chmod(destdir, originalMode, &err)

	contents, err := ioutil.ReadDir(srcdir)
	if err != nil {
		return
	}

	for _, content := range contents {
		cs, cd := filepath.Join(srcdir, content.Name()), filepath.Join(destdir, content.Name())

		if err = switchboard(cs, cd, content); err != nil {
			// If any error, exit immediately
			return
		}
	}

	return
}

func onsymlink(src, dest string, info os.FileInfo) error {
	orig, err := os.Readlink(src)
	if err != nil {
		return err
	}
	info, err = os.Lstat(orig)
	if err != nil {
		return err
	}
	return switchboard(orig, dest, info)
}

// chmod ANYHOW changes file mode,
// with asiging error raised during Chmod,
// BUT respecting the error already reported.
func chmod(dir string, mode os.FileMode, reported *error) {
	if err := os.Chmod(dir, mode); *reported == nil {
		*reported = err
	}
}

// fclose ANYHOW closes file,
// with asiging error raised during Close,
// BUT respecting the error already reported.
func fclose(f *os.File, reported *error) {
	if err := f.Close(); *reported == nil {
		*reported = err
	}
}
