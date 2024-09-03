// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * Portions of this file are based on code originally from:
 * github.com/psanford/memfs
 *
 * Copyright (c) 2021 The memfs Authors. All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 * * Redistributions of source code must retain the above copyright
 * notice, this list of conditions and the following disclaimer.
 * * Redistributions in binary form must reproduce the above
 * copyright notice, this list of conditions and the following disclaimer
 * in the documentation and/or other materials provided with the
 * distribution.
 * * Neither the name of the copyright holder nor the names of its
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

package memfs_test

import (
	"fmt"
	"io/fs"
	"testing"

	"github.com/dpeckett/archivefs/memfs"

	"github.com/stretchr/testify/require"
)

func TestMemFS(t *testing.T) {
	rootFS := memfs.New()

	err := rootFS.MkdirAll("foo/bar", 0777)
	require.NoError(t, err)

	var gotPaths []string

	err = fs.WalkDir(rootFS, ".", func(path string, d fs.DirEntry, err error) error {
		gotPaths = append(gotPaths, path)
		if !d.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
		return nil
	})
	require.NoError(t, err)

	expectPaths := []string{
		".",
		"foo",
		"foo/bar",
	}
	require.Equal(t, expectPaths, gotPaths)

	err = rootFS.WriteFile("foo/baz/buz.txt", []byte("buz"), 0777)
	require.ErrorIs(t, err, fs.ErrNotExist)

	_, err = fs.ReadFile(rootFS, "foo/baz/buz.txt")
	require.ErrorIs(t, err, fs.ErrNotExist)

	body := []byte("baz")
	err = rootFS.WriteFile("foo/bar/baz.txt", body, 0777)
	require.NoError(t, err)

	gotBody, err := fs.ReadFile(rootFS, "foo/bar/baz.txt")
	require.NoError(t, err)

	require.Equal(t, body, gotBody)

	subFS, err := rootFS.Sub("foo/bar")
	require.NoError(t, err)

	gotSubBody, err := fs.ReadFile(subFS, "baz.txt")
	require.NoError(t, err)

	require.Equal(t, body, gotSubBody)

	body = []byte("top_level_file")
	err = rootFS.WriteFile("top_level_file.txt", body, 0777)
	require.NoError(t, err)

	gotBody, err = fs.ReadFile(rootFS, "top_level_file.txt")
	require.NoError(t, err)

	require.Equal(t, body, gotBody)
}
