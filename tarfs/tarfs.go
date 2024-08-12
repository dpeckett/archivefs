// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package tarfs

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/btree"
)

var (
	_ fs.FS        = (*FS)(nil)
	_ fs.ReadDirFS = (*FS)(nil)
	_ fs.StatFS    = (*FS)(nil)
)

type FS struct {
	tree *btree.BTree
}

func Open(ra io.ReaderAt) (*FS, error) {
	r := &readerWithOffset{ra: ra}
	tr := tar.NewReader(r)

	tree := btree.New(2)

	// Add a default root directory entry.
	tree.ReplaceOrInsert(entry{
		Header: tar.Header{
			Typeflag: tar.TypeDir,
			Name:     ".",
			Mode:     0o755,
		},
	})

	for {
		// round to next 512 byte boundary.
		begin := (r.offset + 511) &^ 511

		h, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		switch h.Typeflag {
		case tar.TypeReg, tar.TypeGNUSparse:
			// Discard the file contents (so that the reader is consumed).
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", h.Name, err)
			}
		case tar.TypeDir, tar.TypeSymlink:
			// NOP
		case tar.TypeXGlobalHeader:
			continue // Ignore metadata-only entries.
		case tar.TypeLink:
			// We don't support hard links, so replace them with symlinks.
			h.Typeflag = tar.TypeSymlink
		default:
			return nil, fmt.Errorf("unsupported file type: %s, %c", h.Name, h.Typeflag)
		}

		h.Name = sanitizePath(h.Name)

		// Make archive relative paths absolute.
		if strings.HasPrefix(h.Linkname, "./") {
			h.Linkname = strings.TrimPrefix(h.Linkname, ".")
		}
		h.Linkname = filepath.Clean(h.Linkname)

		// Create a default directory entry for each parent directory.
		for dir := filepath.Dir(h.Name); dir != "." && dir != "/"; dir = filepath.Dir(dir) {
			e := entry{
				Header: tar.Header{
					Typeflag: tar.TypeDir,
					Name:     dir,
					Mode:     0o755,
				},
			}

			// Create a default directory entry if it doesn't exist.
			// Don't worry if we see one later, we'll just overwrite it.
			if tree.Get(e) == nil {
				tree.ReplaceOrInsert(e)
			}
		}

		tree.ReplaceOrInsert(entry{
			Header: *h,
			data: func() io.Reader {
				return io.NewSectionReader(ra, begin, r.offset-begin)
			},
		})
	}

	return &FS{tree: tree}, nil
}

func (fsys *FS) Open(name string) (fs.File, error) {
	path, err := fsys.resolvePath(name)
	if err != nil {
		return nil, err
	}

	e := fsys.tree.Get(entry{Header: tar.Header{Name: path}})
	if e != nil {
		tr := tar.NewReader(e.(entry).data())
		if _, err := tr.Next(); err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", name, err)
		}

		return &file{entry: e.(entry), fsys: fsys, r: tr}, nil
	}

	return nil, fs.ErrNotExist
}

func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	dir, err := fsys.resolvePath(name)
	if err != nil {
		return nil, err
	}

	if dir == "." {
		dir = ""
	}

	var foundDir bool
	var dirEntries []fs.DirEntry

	fsys.tree.AscendGreaterOrEqual(entry{Header: tar.Header{Name: dir}}, func(item btree.Item) bool {
		e := item.(entry)

		if !strings.HasPrefix(e.Name, dir) {
			return false
		}

		foundDir = true

		relPath := strings.TrimPrefix(strings.TrimPrefix(e.Name, dir), "/")
		if relPath == "" || relPath == "." || strings.Contains(relPath, "/") {
			return true
		}

		dirEntries = append(dirEntries, dirEntry{entry: e, fsys: fsys})
		return true
	})

	if !foundDir {
		return nil, fs.ErrNotExist
	}

	return dirEntries, nil
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	path, err := fsys.resolvePath(name)
	if err != nil {
		return nil, err
	}

	e := fsys.tree.Get(entry{Header: tar.Header{Name: path}})
	if e == nil {
		if name == "." {
			return fsys.statEntry(fsys.tree.Get(entry{
				Header: tar.Header{
					Typeflag: tar.TypeDir,
					Name:     ".",
					Mode:     0o777,
				},
			}).(entry))
		}

		return nil, fs.ErrNotExist
	}

	// Use the original name (before it was resolved) for the entry.
	renamedEntry := e.(entry)
	renamedEntry.Name = filepath.Base(name)

	return fsys.statEntry(renamedEntry)
}

// ReadLink returns the destination of the named symbolic link.
// Experimental implementation of fs.ReadLinkFS:
// https://github.com/golang/go/issues/49580
func (fsys *FS) ReadLink(name string) (string, error) {
	dir := filepath.Dir(sanitizePath(name))
	if dir != "." {
		var err error
		dir, err = fsys.resolvePath(dir)
		if err != nil {
			return "", err
		}
	}

	e := fsys.tree.Get(entry{Header: tar.Header{Name: filepath.Join(dir, filepath.Base(name))}})
	if e := e.(entry); e.Typeflag == tar.TypeSymlink {
		return e.Linkname, nil
	}

	return "", fs.ErrInvalid
}

// StatLink returns a FileInfo describing the file without following any symbolic links.
// Experimental implementation of fs.ReadLinkFS:
// https://github.com/golang/go/issues/49580
func (fsys *FS) StatLink(name string) (fs.FileInfo, error) {
	dir := filepath.Dir(sanitizePath(name))
	if dir != "." {
		var err error
		dir, err = fsys.resolvePath(dir)
		if err != nil {
			return nil, err
		}
	}

	e := fsys.tree.Get(entry{Header: tar.Header{Name: filepath.Join(dir, filepath.Base(name))}})
	if e == nil {
		return nil, fs.ErrNotExist
	}

	// Use the original name (before it was resolved) for the entry.
	renamedEntry := e.(entry)
	renamedEntry.Name = filepath.Base(name)

	return fsys.statEntry(renamedEntry)
}

func (fsys *FS) statEntry(e entry) (fs.FileInfo, error) {
	return e.FileInfo(), nil
}

func (fsys *FS) resolvePath(path string) (string, error) {
	if path == "." {
		return ".", nil
	}

	visitedPaths := make(map[string]bool)
	var currentPath string
	for _, comp := range splitPath(sanitizePath(path)) {
		currentPath = filepath.Join(currentPath, comp)

		// Check if the current path has already been visited
		if visitedPaths[currentPath] {
			return "", fmt.Errorf("symlink cycle detected at path: %s", currentPath)
		}
		visitedPaths[currentPath] = true

		e := fsys.tree.Get(entry{Header: tar.Header{Name: currentPath}})
		if e == nil {
			return "", fs.ErrNotExist
		}

		if e.(entry).Typeflag == tar.TypeSymlink {
			linkname := e.(entry).Linkname
			if !filepath.IsAbs(linkname) {
				linkname = filepath.Join(filepath.Dir(currentPath), linkname)
			}

			var err error
			currentPath, err = fsys.resolvePath(linkname)
			if err != nil {
				return "", err
			}
		}
	}

	return currentPath, nil
}

func sanitizePath(name string) string {
	return strings.TrimPrefix(strings.TrimPrefix(filepath.Clean(strings.TrimSpace(name)), "/"), "./")
}

func splitPath(path string) []string {
	var components []string
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part != "" {
			components = append(components, part)
		}
	}
	return components
}

type file struct {
	entry
	fsys *FS
	r    io.Reader
}

func (f *file) Stat() (fs.FileInfo, error) {
	return f.fsys.statEntry(f.entry)
}

func (f *file) Read(p []byte) (n int, err error) {
	return f.r.Read(p)
}

func (f *file) Close() error {
	return nil
}

type entry struct {
	tar.Header
	data func() io.Reader
}

func (e entry) Less(than btree.Item) bool {
	return strings.Compare(e.Name, than.(entry).Name) < 0
}

type dirEntry struct {
	entry
	fsys *FS
}

func (de dirEntry) Name() string {
	return filepath.Base(de.entry.Name)
}

func (de dirEntry) IsDir() bool {
	return de.entry.Typeflag == tar.TypeDir
}

func (de dirEntry) Type() fs.FileMode {
	mode := fs.FileMode(de.entry.Mode) & fs.ModePerm

	// Handle setuid, setgid, and sticky bits.
	if de.entry.Mode&syscall.S_ISVTX != 0 {
		mode |= fs.ModeSticky
	}
	if de.entry.Mode&syscall.S_ISUID != 0 {
		mode |= fs.ModeSetuid
	}
	if de.entry.Mode&syscall.S_ISGID != 0 {
		mode |= fs.ModeSetgid
	}

	switch de.entry.Typeflag {
	case tar.TypeReg:
		return mode
	case tar.TypeSymlink:
		return mode | fs.ModeSymlink
	case tar.TypeChar:
		return mode | fs.ModeCharDevice
	case tar.TypeBlock:
		return mode | fs.ModeDevice
	case tar.TypeDir:
		return mode | fs.ModeDir
	case tar.TypeFifo:
		return mode | fs.ModeNamedPipe
	default:
		return mode | fs.ModeIrregular
	}
}

func (de dirEntry) Info() (fs.FileInfo, error) {
	return de.fsys.statEntry(de.entry)
}

// readerWithOffset is a wrapper around io.ReaderAt that keeps track of the current offset.
type readerWithOffset struct {
	ra     io.ReaderAt
	offset int64
}

func (f *readerWithOffset) Read(p []byte) (n int, err error) {
	n, err = f.ra.ReadAt(p, f.offset)
	f.offset += int64(n)
	return
}
