// Copyright 2026 CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package embed

import (
	"errors"
	iofs "io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"syscall"
)

// rootedFS serves the files of fsys, the directory of an embedding CUE
// file, only when their paths stay within the module root on disk once
// symbolic links are resolved.
//
// The paths of an @embed attribute are checked lexically, but the OS file
// system follows symbolic links, so a link within the module could make
// any file on the machine part of the module's evaluation.
//
// Each path is resolved with [os.Root.Stat] before fsys serves it. A path
// that does not exist on disk is left to fsys, which may hold it in an
// overlay. Files are still read from fsys, so that an overlay keeps
// precedence over the disk, but a path is refused if it resolves outside
// the root on disk, also when an overlay holds it. The check and the read
// are not atomic, which only matters to someone able to change the
// module's files while it is being evaluated.
type rootedFS struct {
	fsys iofs.FS

	// root is the module root on disk.
	root *os.Root

	// dir is the directory of fsys within root, slash separated.
	dir string

	// escaped records the first path refused for resolving outside root.
	// [iofs.Glob] drops the errors of the directories it reads, so a glob
	// checks it after matching.
	escaped *escapeError
}

var _ interface {
	iofs.StatFS
	iofs.ReadDirFS
} = (*rootedFS)(nil)

// An escapeError reports a path that resolves outside the module root,
// which os.Root also reports for an absolute link.
type escapeError struct {
	path string
}

func (e *escapeError) Error() string {
	return strconv.Quote(e.path) + " goes through a symbolic link that is absolute or leads outside the module root"
}

// check reports an error if name, which is relative to fsys,
// cannot be shown to resolve within the module root.
func (r *rootedFS) check(op, name string) error {
	if !iofs.ValidPath(name) {
		return &iofs.PathError{Op: op, Path: name, Err: iofs.ErrInvalid}
	}
	_, err := r.root.Stat(filepath.FromSlash(path.Join(r.dir, name)))
	if err == nil || errors.Is(err, iofs.ErrNotExist) {
		return nil
	}
	// A system call failing, as for a loop of links, is reported as such.
	// Otherwise the path left the root: os.Root reports that with an
	// error of its own, as for a link to a parent of the root or an
	// absolute link.
	if errno := syscall.Errno(0); errors.As(err, &errno) {
		return &iofs.PathError{Op: op, Path: name, Err: err}
	}
	e := &escapeError{path: name}
	if r.escaped == nil {
		r.escaped = e
	}
	return e
}

func (r *rootedFS) Open(name string) (iofs.File, error) {
	if err := r.check("open", name); err != nil {
		return nil, err
	}
	return r.fsys.Open(name)
}

func (r *rootedFS) Stat(name string) (iofs.FileInfo, error) {
	if err := r.check("stat", name); err != nil {
		return nil, err
	}
	return iofs.Stat(r.fsys, name)
}

func (r *rootedFS) ReadDir(name string) ([]iofs.DirEntry, error) {
	if err := r.check("readdir", name); err != nil {
		return nil, err
	}
	return iofs.ReadDir(r.fsys, name)
}
