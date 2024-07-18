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

package copyfs

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
)

func TestCopyFS(t *testing.T) {
	t.Parallel()

	// Test with disk filesystem.
	forceMFTUpdateOnWindows(t, "./testdata/dirfs")
	fsys := os.DirFS("./testdata/dirfs")
	tmpDir := t.TempDir()
	if err := CopyFS(tmpDir, fsys); err != nil {
		t.Fatal("CopyFS:", err)
	}
	forceMFTUpdateOnWindows(t, tmpDir)
	tmpFsys := os.DirFS(tmpDir)
	if err := fstest.TestFS(tmpFsys, "a", "b", "dir/x"); err != nil {
		t.Fatal("TestFS:", err)
	}
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		newData, err := fs.ReadFile(tmpFsys, path)
		if err != nil {
			return err
		}
		if !bytes.Equal(data, newData) {
			return errors.New("file " + path + " contents differ")
		}
		return nil
	}); err != nil {
		t.Fatal("comparing two directories:", err)
	}

	// Test with memory filesystem.
	fsys = fstest.MapFS{
		"william":    {Data: []byte("Shakespeare\n")},
		"carl":       {Data: []byte("Gauss\n")},
		"daVinci":    {Data: []byte("Leonardo\n")},
		"einstein":   {Data: []byte("Albert\n")},
		"dir/newton": {Data: []byte("Sir Isaac\n")},
	}
	tmpDir = t.TempDir()
	if err := CopyFS(tmpDir, fsys); err != nil {
		t.Fatal("CopyFS:", err)
	}
	forceMFTUpdateOnWindows(t, tmpDir)
	tmpFsys = os.DirFS(tmpDir)
	if err := fstest.TestFS(tmpFsys, "william", "carl", "daVinci", "einstein", "dir/newton"); err != nil {
		t.Fatal("TestFS:", err)
	}
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		newData, err := fs.ReadFile(tmpFsys, path)
		if err != nil {
			return err
		}
		if !bytes.Equal(data, newData) {
			return errors.New("file " + path + " contents differ")
		}
		return nil
	}); err != nil {
		t.Fatal("comparing two directories:", err)
	}
}

func forceMFTUpdateOnWindows(t *testing.T, path string) {
	t.Helper()

	if runtime.GOOS != "windows" {
		return
	}

	// On Windows, we force the MFT to update by reading the actual metadata from GetFileInformationByHandle and then
	// explicitly setting that. Otherwise it might get out of sync with FindFirstFile. See golang.org/issues/42637.
	if err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		info, err := d.Info()
		if err != nil {
			t.Fatal(err)
		}
		stat, err := os.Stat(path) // This uses GetFileInformationByHandle internally.
		if err != nil {
			t.Fatal(err)
		}
		if stat.ModTime() == info.ModTime() {
			return nil
		}
		if err := os.Chtimes(path, stat.ModTime(), stat.ModTime()); err != nil {
			t.Log(err) // We only log, not die, in case the test directory is not writable.
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
