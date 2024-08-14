// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package archivefs

import (
	"io/fs"
)

// ReadLinkFS is the interface that a file system must implement to
// support the ReadLink and StatLink methods.
// This is an experimental implementation of the API described in:
// https://github.com/golang/go/issues/49580
type ReadLinkFS interface {
	fs.FS

	// ReadLink returns the destination of the named symbolic link.
	ReadLink(name string) (string, error)

	// StatLink returns a FileInfo describing the file without following any symbolic links.
	StatLink(name string) (fs.FileInfo, error)
}
