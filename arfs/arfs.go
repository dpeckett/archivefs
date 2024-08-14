// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * Portions of this file are based on code originally from: github.com/paultag/go-debian
 *
 * Copyright (c) Paul R. Tagliamonte <paultag@debian.org>, 2015
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

package arfs

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	_ fs.FS        = (*FS)(nil)
	_ fs.ReadDirFS = (*FS)(nil)
	_ fs.StatFS    = (*FS)(nil)
)

// FS is a filesystem that represents a Debian .deb flavored `ar(1)` archive.
type FS struct {
	entries map[string]*Entry
}

// Open a new `ar(1)` archive from the given `io.ReaderAt`.
func Open(ra io.ReaderAt) (*FS, error) {
	// Validate the archive header.
	offset, err := checkAr(ra)
	if err != nil {
		return nil, err
	}

	// Read the entries from the archive.
	entries := map[string]*Entry{}
	for {
		line := make([]byte, 60)

		n, err := ra.ReadAt(line, offset)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		if n == 1 && line[0] == '\n' {
			return nil, io.EOF
		}
		if n != 60 {
			return nil, errors.New("short read")
		}

		e, err := parseArEntry(line)
		if err != nil {
			return nil, err
		}

		if strings.Contains(e.Filename, "/") {
			return nil, fmt.Errorf("invalid filename: %s", e.Filename)
		}

		begin := offset + int64(n)
		e.data = func() io.Reader {
			return io.NewSectionReader(ra, begin, e.FileSize)
		}
		offset += int64(n) + e.FileSize + (e.FileSize % 2)

		entries[e.Filename] = e
	}

	return &FS{entries: entries}, nil
}

// Open a file from the archive.
func (fsys *FS) Open(name string) (fs.File, error) {
	name = sanitizePath(name)

	e, ok := fsys.entries[name]
	if !ok {
		return nil, fs.ErrNotExist
	}

	return &file{Entry: e, Reader: e.data()}, nil
}

// ReadDir reads the contents of the archive.
func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	name = sanitizePath(name)
	if name != "" {
		return nil, errors.New("ar does not support directories")
	}

	var dirEntries []fs.DirEntry
	for _, dirent := range fsys.entries {
		dirEntries = append(dirEntries, dirent)
	}

	return dirEntries, nil
}

// Stat a file in the archive.
func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	name = sanitizePath(name)

	if name == "" {
		return &Entry{
			Filename: ".",
			FileMode: fs.ModeDir,
		}, nil
	}

	e, ok := fsys.entries[name]
	if !ok {
		return nil, fs.ErrNotExist
	}

	return e, nil
}

// Take the AR format line, and create an ArEntry (without .Data set)
// to be returned to the user later.
func parseArEntry(line []byte) (*Entry, error) {
	if len(line) != 60 {
		return nil, errors.New("malformed file entry line length")
	}

	if line[58] != 0x60 && line[59] != 0x0A {
		return nil, errors.New("malformed file entry line endings")
	}

	mode, err := strconv.ParseUint(strings.TrimSpace(string(line[40:48])), 8, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file mode: %w", err)
	}

	// The tar package has a handy conversion function for the unix file mode bits.
	fileMode := (&tar.Header{Mode: int64(mode)}).FileInfo().Mode()

	e := Entry{
		Filename: sanitizePath(string(line[0:16])),
		FileMode: fileMode,
	}

	for target, value := range map[*int64][]byte{
		&e.Timestamp: line[16:28],
		&e.Uid:       line[28:34],
		&e.Gid:       line[34:40],
		&e.FileSize:  line[48:58],
	} {
		input := strings.TrimSpace(string(value))
		if input == "" {
			continue
		}

		intValue, err := strconv.Atoi(input)
		if err != nil {
			return nil, fmt.Errorf("failed to parse entry value: %w", err)
		}
		*target = int64(intValue)
	}

	return &e, nil
}

// Given a brand spank'n new os.File entry, go ahead and make sure it looks
// like an `ar(1)` archive, and not some random file.
func checkAr(ra io.ReaderAt) (int64, error) {
	header := make([]byte, 8)
	if _, err := ra.ReadAt(header, 0); err != nil {
		return 0, err
	}
	if string(header) != "!<arch>\n" {
		return 0, errors.New("invalid ar archive header")
	}
	return int64(len(header)), nil
}

func sanitizePath(name string) string {
	return strings.TrimPrefix(strings.TrimPrefix(filepath.Clean(filepath.ToSlash(strings.TrimSpace(name))), "."), "/")
}

type file struct {
	*Entry
	io.Reader
}

func (f *file) Stat() (fs.FileInfo, error) {
	return f.Entry, nil
}

func (f *file) Close() error {
	return nil
}

type Entry struct {
	Filename  string
	Timestamp int64
	Uid       int64
	Gid       int64
	FileMode  fs.FileMode
	FileSize  int64
	data      func() io.Reader
}

func (e *Entry) Name() string {
	return filepath.Base(e.Filename)
}

func (e *Entry) Size() int64 {
	return e.FileSize
}

func (e *Entry) Mode() fs.FileMode {
	return e.FileMode
}

func (e *Entry) ModTime() time.Time {
	return time.Unix(e.Timestamp, 0)
}

func (e *Entry) IsDir() bool {
	return e.FileMode&fs.ModeDir != 0
}

func (e *Entry) Sys() any {
	return e
}

func (e *Entry) Type() fs.FileMode {
	return e.FileMode & fs.ModeType
}

func (e *Entry) Info() (fs.FileInfo, error) {
	return e, nil
}
