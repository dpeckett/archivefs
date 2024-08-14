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
	"slices"
	"strings"

	"github.com/dpeckett/archivefs"
)

var (
	_ fs.FS                = (*FS)(nil)
	_ fs.ReadDirFS         = (*FS)(nil)
	_ fs.StatFS            = (*FS)(nil)
	_ archivefs.ReadLinkFS = (*FS)(nil)
)

type FS struct {
	root dirent
}

func Open(ra io.ReaderAt) (*FS, error) {
	r := &readerWithOffset{ra: ra}
	tr := tar.NewReader(r)

	dirents := map[string]*dirent{}
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

		// there might be a junk root entry.
		if h.Name == "" {
			continue
		}

		// Make archive relative paths absolute.
		if h.Typeflag == tar.TypeSymlink {
			if strings.HasPrefix(h.Linkname, "./") {
				h.Linkname = strings.TrimPrefix(h.Linkname, ".")
			}
			h.Linkname = filepath.Clean(h.Linkname)
		}

		// Create a default directory entry for each parent directory.
		for dir := filepath.Dir(h.Name); dir != "." && dir != "/"; dir = filepath.Dir(dir) {
			// Create a default directory entry if it doesn't exist.
			// Don't worry if we see one later, we'll just overwrite it.
			dirents[dir] = &dirent{
				Header: tar.Header{
					Typeflag: tar.TypeDir,
					Name:     dir,
					Mode:     0o755,
				},
			}
		}

		size := r.offset - begin

		dirents[h.Name] = &dirent{
			Header: *h,
			data: func() io.Reader {
				return io.NewSectionReader(ra, begin, size)
			},
		}
	}

	var paths []string
	for path := range dirents {
		paths = append(paths, path)
	}

	slices.Sort(paths)

	root := dirent{
		Header: tar.Header{
			Typeflag: tar.TypeDir,
			Name:     ".",
			Mode:     0o755,
		},
	}

	for _, path := range paths {
		d := dirents[path]

		dir, err := resolve(&root, filepath.Dir(path))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve directory %q: %w", filepath.Dir(path), err)
		}

		dir.addChild(d)
	}

	return &FS{root: root}, nil
}

func (fsys *FS) Open(name string) (fs.File, error) {
	d, err := resolve(&fsys.root, name)
	if err != nil {
		return nil, err
	}

	tr := tar.NewReader(d.data())
	if _, err := tr.Next(); err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", name, err)
	}

	return &file{dirent: d, r: tr}, nil
}

func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	d, err := resolve(&fsys.root, name)
	if err != nil {
		return nil, err
	}

	var children []fs.DirEntry
	for _, child := range d.children {
		children = append(children, child)
	}

	slices.SortFunc(children, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	return children, nil
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	if sanitizePath(name) == "" {
		d := &dirent{
			Header: tar.Header{
				Typeflag: tar.TypeDir,
				Name:     ".",
				Mode:     0o755,
			},
		}

		return d.Info()
	}

	d, err := resolve(&fsys.root, name)
	if err != nil {
		return nil, err
	}

	// Use the original name (as we may be resolving a symlink).
	renamed := *d
	renamed.Header.Name = name

	return renamed.Info()
}

// ReadLink returns the destination of the named symbolic link.
// Experimental implementation of fs.ReadLinkFS:
// https://github.com/golang/go/issues/49580
func (fsys *FS) ReadLink(name string) (string, error) {
	d, err := resolve(&fsys.root, filepath.Dir(name))
	if err != nil {
		return "", err
	}

	d, found := d.findChild(filepath.Base(name))
	if !found {
		return "", fs.ErrNotExist
	}

	if d.Type()&fs.ModeSymlink == 0 {
		return "", fs.ErrInvalid
	}

	return d.Linkname, nil
}

// StatLink returns a FileInfo describing the file without following any symbolic links.
// Experimental implementation of fs.ReadLinkFS:
// https://github.com/golang/go/issues/49580
func (fsys *FS) StatLink(name string) (fs.FileInfo, error) {
	d, err := resolve(&fsys.root, filepath.Dir(name))
	if err != nil {
		return nil, err
	}

	d, found := d.findChild(filepath.Base(name))
	if !found {
		return nil, fs.ErrNotExist
	}

	return d.Info()
}

func resolve(root *dirent, name string) (*dirent, error) {
	d := root

	name = sanitizePath(name)
	if name == "" {
		return d, nil
	}

	for _, component := range strings.Split(name, "/") {
		var found bool
		d, found = d.findChild(component)
		if !found {
			return nil, fs.ErrNotExist
		}

		if d.Type()&fs.ModeSymlink != 0 {
			// Resolve the symlink.
			target := d.Linkname

			var err error
			if !filepath.IsAbs(target) && d.parent != nil {
				d, err = resolve(d.parent, target)
				if err != nil {
					return nil, err
				}
			} else {
				// The target is an absolute path or the dirent is the root dirent.
				d, err = resolve(root, target)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return d, nil
}

func sanitizePath(name string) string {
	return strings.TrimPrefix(strings.TrimPrefix(filepath.Clean(filepath.ToSlash(strings.TrimSpace(name))), "."), "/")
}

type file struct {
	*dirent
	r io.Reader
}

func (f *file) Stat() (fs.FileInfo, error) {
	return f.Info()
}

func (f *file) Read(p []byte) (n int, err error) {
	return f.r.Read(p)
}

func (f *file) Close() error {
	return nil
}

var _ fs.DirEntry = &dirent{}

type dirent struct {
	tar.Header
	parent   *dirent
	children map[string]*dirent
	data     func() io.Reader
}

func (d *dirent) findChild(name string) (*dirent, bool) {
	c, ok := d.children[name]
	return c, ok
}

func (d *dirent) addChild(child *dirent) {
	if d.children == nil {
		d.children = make(map[string]*dirent)
	}

	c := *child
	c.parent = d

	// do we already have a child with the same name?
	if existing, ok := d.children[c.Name()]; ok {
		c.children = existing.children
	}

	d.children[c.Name()] = &c
}

func (d *dirent) Name() string {
	return filepath.Base(d.Header.Name)
}

func (d *dirent) IsDir() bool {
	return d.Typeflag == tar.TypeDir
}

func (d *dirent) Type() fs.FileMode {
	switch d.Typeflag {
	case tar.TypeReg:
		return 0
	case tar.TypeSymlink:
		return fs.ModeSymlink
	case tar.TypeChar:
		return fs.ModeCharDevice
	case tar.TypeBlock:
		return fs.ModeDevice
	case tar.TypeDir:
		return fs.ModeDir
	case tar.TypeFifo:
		return fs.ModeNamedPipe
	default:
		return fs.ModeIrregular
	}
}

func (d *dirent) Info() (fs.FileInfo, error) {
	return d.FileInfo(), nil
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
