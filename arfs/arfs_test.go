// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package arfs_test

import (
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/dpeckett/archivefs/arfs"

	"github.com/stretchr/testify/require"
)

func TestArFS(t *testing.T) {
	f, err := os.Open("testdata/multi_archive.a")
	require.NoError(t, err)

	fsys, err := arfs.Open(f)
	require.NoError(t, err)

	// Test that we can stat the file.
	fi, err := fsys.Stat("hello.txt")
	require.NoError(t, err)

	require.Equal(t, "hello.txt", fi.Name())
	require.Equal(t, int64(1361157466), fi.ModTime().Unix())
	require.Equal(t, int64(501), fi.Sys().(*arfs.Entry).Uid)
	require.Equal(t, int64(20), fi.Sys().(*arfs.Entry).Gid)
	require.Equal(t, fs.FileMode(33188), fi.Mode())

	// Now, test that we can read the contents of the file.
	arFile, err := fsys.Open("hello.txt")
	require.NoError(t, err)

	content, err := io.ReadAll(arFile)
	require.NoError(t, err)

	require.Equal(t, "Hello world!\n", string(content))

	// Now, test that we can read the contents of the second file.
	arFile, err = fsys.Open("lamp.txt")
	require.NoError(t, err)

	content, err = io.ReadAll(arFile)
	require.NoError(t, err)

	require.Equal(t, "I love lamp.\n", string(content))

	// List the files in the archive.
	dir, err := fsys.ReadDir(".")
	require.NoError(t, err)

	require.Len(t, dir, 2)

	require.Equal(t, "hello.txt", dir[0].Name())
	require.Equal(t, "lamp.txt", dir[1].Name())
}
