// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package arfs

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strconv"
)

// Create creates an ar(1) archive from the given filesystem.
func Create(dst io.Writer, src fs.FS) error {
	// Write the ar(1) magic header
	if _, err := io.WriteString(dst, "!<arch>\n"); err != nil {
		return err
	}

	entries, err := fs.ReadDir(src, ".")
	if err != nil {
		return err
	}

	for _, d := range entries {
		if d.IsDir() {
			return errors.New("directories are not supported")
		}

		if d.Type()&fs.ModeSymlink != 0 {
			return errors.New("symlinks are not supported")
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}

		// Write ar(1) header for the file
		if err := writeArHeader(dst, hdr); err != nil {
			return err
		}

		// Write file data (if not a directory)
		if !d.IsDir() {
			if err := writeArData(dst, src, d.Name(), hdr.Size); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeArHeader(w io.Writer, hdr *tar.Header) error {
	name := sanitizePath(hdr.Name)
	if len(name) > 16 {
		return fmt.Errorf("file name too long: %s", name)
	}

	// Construct the ar(1) header
	arHeader := fmt.Sprintf(
		"%-16s%-12s%-6s%-6s%-8s%-10s`\n",
		name,
		strconv.Itoa(int(hdr.ModTime.Unix())),
		strconv.Itoa(hdr.Uid),
		strconv.Itoa(hdr.Gid),
		fmt.Sprintf("%07o", hdr.Mode),
		fmt.Sprintf("%-10d", hdr.Size),
	)

	// Write the ar header to the output
	if _, err := io.WriteString(w, arHeader); err != nil {
		return err
	}

	return nil
}

func writeArData(w io.Writer, src fs.FS, path string, size int64) error {
	// Open the file to read its data
	file, err := src.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy the file data to the output writer
	n, err := io.Copy(w, file)
	if err != nil {
		return err
	}

	// Handle padding if the file size is odd (ar format requires even size)
	if n%2 != 0 {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}

	return nil
}
