// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * Portions of this file are based on code originally from: https://github.com/golang/go
 *
 * Copyright (c) 2009 The Go Authors. All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 *    * Redistributions of source code must retain the above copyright
 * notice, this list of conditions and the following disclaimer.
 *    * Redistributions in binary form must reproduce the above
 * copyright notice, this list of conditions and the following disclaimer
 * in the documentation and/or other materials provided with the
 * distribution.
 *    * Neither the name of Google Inc. nor the names of its
 * contributors may be used to endorse or promote products derived from
 * this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

// Package copyfs provides a backport of the Go 1.23 os.CopyFS() function.
package copyfs

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyFS copies the file system fsys into the directory dir,
// creating dir if necessary.
//
// Newly created directories and files have their default modes
// where any bits from the file in fsys that are not part of the
// standard read, write, and execute permissions will be zeroed
// out, and standard read and write permissions are set for owner,
// group, and others while retaining any existing execute bits from
// the file in fsys.
//
// Symbolic links in fsys are not supported, a *PathError with Err set
// to ErrInvalid is returned on symlink.
//
// Copying stops at and returns the first error encountered.
func CopyFS(dir string, fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		fpath, err := localize(path)
		if err != nil {
			return err
		}
		newPath := filepath.Join(dir, fpath)
		if d.IsDir() {
			return os.MkdirAll(newPath, 0o777)
		}

		// TODO(panjf2000): handle symlinks with the help of fs.ReadLinkFS
		// 		once https://go.dev/issue/49580 is done.
		//		we also need filepathlite.IsLocal from https://go.dev/cl/564295.
		if !d.Type().IsRegular() {
			return &os.PathError{Op: "CopyFS", Path: path, Err: fs.ErrInvalid}
		}

		r, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		info, err := r.Stat()
		if err != nil {
			return err
		}
		w, err := os.OpenFile(newPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666|info.Mode()&0o777)
		if err != nil {
			return err
		}

		if _, err := io.Copy(w, r); err != nil {
			_ = w.Close()
			return &os.PathError{Op: "Copy", Path: newPath, Err: err}
		}
		return w.Close()
	})
}
