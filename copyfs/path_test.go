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
	"path/filepath"
	"runtime"
	"testing"
)

type LocalizeTest struct {
	path string
	want string
}

var localizetests = []LocalizeTest{
	{"", ""},
	{".", "."},
	{"..", ""},
	{"a/..", ""},
	{"/", ""},
	{"/a", ""},
	{"a\xffb", ""},
	{"a/", ""},
	{"a/./b", ""},
	{"\x00", ""},
	{"a", "a"},
	{"a/b/c", "a/b/c"},
}

var unixlocalizetests = []LocalizeTest{
	{"#a", "#a"},
	{`a\b:c`, `a\b:c`},
}

var winlocalizetests = []LocalizeTest{
	{"#a", "#a"},
	{"c:", ""},
	{`a\b`, ""},
	{`a:b`, ""},
	{`a/b:c`, ""},
	{`NUL`, ""},
	{`a/NUL`, ""},
	{`./com1`, ""},
	{`a/nul/b`, ""},
}

func TestLocalize(t *testing.T) {
	tests := localizetests
	switch runtime.GOOS {
	case "windows":
		tests = append(tests, winlocalizetests...)
		for i := range tests {
			tests[i].want = filepath.FromSlash(tests[i].want)
		}
	default:
		tests = append(tests, unixlocalizetests...)
	}
	for _, test := range tests {
		got, err := localize(test.path)
		wantErr := "<nil>"
		if test.want == "" {
			wantErr = "error"
		}
		if got != test.want || ((err == nil) != (test.want != "")) {
			t.Errorf("IsLocal(%q) = %q, %v want %q, %v", test.path, got, err, test.want, wantErr)
		}
	}
}
