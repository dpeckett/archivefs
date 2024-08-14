// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package testutil

import (
	"io"
	"io/fs"
	"path/filepath"

	"github.com/rogpeppe/go-internal/dirhash"
)

func HashFS(fsys fs.FS) (string, error) {
	var files []string
	err := fs.WalkDir(fsys, ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		files = append(files, filepath.ToSlash(file))
		return nil
	})
	if err != nil {
		return "", err
	}

	return dirhash.Hash1(files, func(name string) (io.ReadCloser, error) {
		return fsys.Open(name)
	})
}
