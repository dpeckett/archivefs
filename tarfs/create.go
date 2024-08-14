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
	"io"
	"io/fs"

	"github.com/dpeckett/archivefs"
)

// Create creates a tar archive from the given filesystem.
func Create(dst io.Writer, src fs.FS) error {
	tw := tar.NewWriter(dst)
	defer tw.Close()

	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// There's an implicit root directory in a tar archive, so we don't need to
		// write it (and it can confuse unarchivers).
		if path == "." {
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}

		var link string
		if d.Type()&fs.ModeSymlink != 0 {
			linkFS, ok := src.(archivefs.ReadLinkFS)
			if !ok {
				return errors.New("source FS does not support symlinks")
			}

			link, err = linkFS.ReadLink(path)
			if err != nil {
				return err
			}
		}

		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return err
		}
		hdr.Name = path

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !d.Type().IsRegular() {
			return nil
		}

		f, err := src.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(tw, f)
		_ = f.Close()
		if err != nil {
			return err
		}

		return nil
	})
}
