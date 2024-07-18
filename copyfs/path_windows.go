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
	"strings"
	"syscall"
)

func osLocalize(path string) (string, error) {
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case ':', '\\', 0:
			return "", errInvalidPath
		}
	}
	containsSlash := false
	for p := path; p != ""; {
		// Find the next path element.
		var element string
		i := strings.IndexByte(p, '/')
		if i < 0 {
			element = p
			p = ""
		} else {
			containsSlash = true
			element = p[:i]
			p = p[i+1:]
		}
		if isReservedName(element) {
			return "", errInvalidPath
		}
	}
	if containsSlash {
		// We can't depend on strings, so substitute \ for / manually.
		buf := []byte(path)
		for i, b := range buf {
			if b == '/' {
				buf[i] = '\\'
			}
		}
		path = string(buf)
	}
	return path, nil
}

// isReservedName reports if name is a Windows reserved device name.
// It does not detect names with an extension, which are also reserved on some Windows versions.
//
// For details, search for PRN in
// https://docs.microsoft.com/en-us/windows/desktop/fileio/naming-a-file.
func isReservedName(name string) bool {
	// Device names can have arbitrary trailing characters following a dot or colon.
	base := name
	for i := 0; i < len(base); i++ {
		switch base[i] {
		case ':', '.':
			base = base[:i]
		}
	}
	// Trailing spaces in the last path element are ignored.
	for len(base) > 0 && base[len(base)-1] == ' ' {
		base = base[:len(base)-1]
	}
	if !isReservedBaseName(base) {
		return false
	}
	if len(base) == len(name) {
		return true
	}
	// The path element is a reserved name with an extension.
	// Some Windows versions consider this a reserved name,
	// while others do not. Use FullPath to see if the name is
	// reserved.
	if p, _ := syscall.FullPath(name); len(p) >= 4 && p[:4] == `\\.\` {
		return true
	}
	return false
}

func isReservedBaseName(name string) bool {
	if len(name) == 3 {
		switch strings.ToUpper(name)[:3] {
		case "CON", "PRN", "AUX", "NUL":
			return true
		}
	}
	if len(name) >= 4 {
		switch strings.ToUpper(name)[:3] {
		case "COM", "LPT":
			if len(name) == 4 && '1' <= name[3] && name[3] <= '9' {
				return true
			}
			// Superscript ¹, ², and ³ are considered numbers as well.
			switch name[3:] {
			case "\u00b2", "\u00b3", "\u00b9":
				return true
			}
			return false
		}
	}

	// Passing CONIN$ or CONOUT$ to CreateFile opens a console handle.
	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea#consoles
	//
	// While CONIN$ and CONOUT$ aren't documented as being files,
	// they behave the same as CON. For example, ./CONIN$ also opens the console input.
	if len(name) == 6 && name[5] == '$' && strings.EqualFold(name, "CONIN$") {
		return true
	}
	if len(name) == 7 && name[6] == '$' && strings.EqualFold(name, "CONOUT$") {
		return true
	}
	return false
}
