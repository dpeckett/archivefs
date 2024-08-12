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

package tarfs_test

import (
	"archive/tar"
	"crypto/md5"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dpeckett/archivefs/tarfs"
	"github.com/rogpeppe/go-internal/dirhash"
	"github.com/stretchr/testify/require"
)

func TestTarFS(t *testing.T) {
	type file struct {
		Name       string
		Size       int64
		Mode       fs.FileMode
		Uid        int
		Gid        int
		AccessTime time.Time
		ModTime    time.Time
		ChangeTime time.Time
		Xattrs     map[string]string
		Linkname   string
		DevMajor   int64
		DevMinor   int64
		IsDir      bool
		IsSymlink  bool
		Sum        string // MD5 checksum of file, leave as empty string if not checked
	}

	vectors := []struct {
		input string // Test input file
		files []file // Expected files in the archive
	}{{
		input: "testdata/gnu.tar",
		files: []file{{
			Name:    "small.txt",
			Size:    5,
			Mode:    0640,
			Uid:     73025,
			Gid:     5000,
			ModTime: time.Unix(1244428340, 0),
			Sum:     "e38b27eaccb4391bdec553a7f3ae6b2f",
		}, {
			Name:    "small2.txt",
			Size:    11,
			Mode:    0640,
			Uid:     73025,
			Gid:     5000,
			ModTime: time.Unix(1244436044, 0),
			Sum:     "c65bd2e50a56a2138bf1716f2fd56fe9",
		}},
	}, {
		input: "testdata/sparse-formats.tar",
		files: []file{{
			Name:    "sparse-gnu",
			Mode:    420,
			Uid:     1000,
			Gid:     1000,
			Size:    200,
			ModTime: time.Unix(1392395740, 0),
			Sum:     "6f53234398c2449fe67c1812d993012f",
		}, {
			Name:    "sparse-posix-0.0",
			Mode:    420,
			Uid:     1000,
			Gid:     1000,
			Size:    200,
			ModTime: time.Unix(1392342187, 0),
			Sum:     "6f53234398c2449fe67c1812d993012f",
		}, {
			Name:    "sparse-posix-0.1",
			Mode:    420,
			Uid:     1000,
			Gid:     1000,
			Size:    200,
			ModTime: time.Unix(1392340456, 0),
			Sum:     "6f53234398c2449fe67c1812d993012f",
		}, {
			Name:    "sparse-posix-1.0",
			Mode:    420,
			Uid:     1000,
			Gid:     1000,
			Size:    200,
			ModTime: time.Unix(1392337404, 0),
			Sum:     "6f53234398c2449fe67c1812d993012f",
		}, {
			Name:    "end",
			Mode:    420,
			Uid:     1000,
			Gid:     1000,
			Size:    4,
			ModTime: time.Unix(1392398319, 0),
			Sum:     "b0061974914468de549a2af8ced10316",
		}},
	}, {
		input: "testdata/star.tar",
		files: []file{{
			Name:       "small.txt",
			Mode:       0640,
			Uid:        73025,
			Gid:        5000,
			Size:       5,
			ModTime:    time.Unix(1244592783, 0),
			AccessTime: time.Unix(1244592783, 0),
			ChangeTime: time.Unix(1244592783, 0),
		}, {
			Name:       "small2.txt",
			Mode:       0640,
			Uid:        73025,
			Gid:        5000,
			Size:       11,
			ModTime:    time.Unix(1244592783, 0),
			AccessTime: time.Unix(1244592783, 0),
			ChangeTime: time.Unix(1244592783, 0),
		}},
	}, {
		input: "testdata/v7.tar",
		files: []file{{
			Name:    "small.txt",
			Mode:    0444,
			Uid:     73025,
			Gid:     5000,
			Size:    5,
			ModTime: time.Unix(1244593104, 0),
		}, {
			Name:    "small2.txt",
			Mode:    0444,
			Uid:     73025,
			Gid:     5000,
			Size:    11,
			ModTime: time.Unix(1244593104, 0),
		}},
	}, {
		input: "testdata/pax.tar",
		files: []file{{
			Name:       "a/123456789101112131415161718192021222324252627282930313233343536373839404142434445464748495051525354555657585960616263646566676869707172737475767778798081828384858687888990919293949596979899100",
			Mode:       0664,
			Uid:        1000,
			Gid:        1000,
			Size:       7,
			ModTime:    time.Unix(1350244992, 23960108),
			ChangeTime: time.Unix(1350244992, 23960108),
			AccessTime: time.Unix(1350244992, 23960108),
		}, {
			Name:       "a/b",
			Mode:       0777,
			Uid:        1000,
			Gid:        1000,
			Size:       0,
			ModTime:    time.Unix(1350266320, 910238425),
			ChangeTime: time.Unix(1350266320, 910238425),
			AccessTime: time.Unix(1350266320, 910238425),
			Linkname:   "123456789101112131415161718192021222324252627282930313233343536373839404142434445464748495051525354555657585960616263646566676869707172737475767778798081828384858687888990919293949596979899100",
			IsSymlink:  true,
		}},
	}, {
		input: "testdata/pax-pos-size-file.tar",
		files: []file{{
			Name:    "foo",
			Mode:    0640,
			Uid:     319973,
			Gid:     5000,
			Size:    999,
			ModTime: time.Unix(1442282516, 0),
			Sum:     "0afb597b283fe61b5d4879669a350556",
		}},
	}, {
		input: "testdata/pax-records.tar",
		files: []file{{
			Name:    "file",
			ModTime: time.Unix(0, 0),
		}},
	}, {
		input: "testdata/pax-global-records.tar",
		files: []file{{
			Name:    "file1",
			ModTime: time.Unix(0, 0),
		}, {
			Name:    "file2",
			ModTime: time.Unix(0, 0),
		}, {
			Name:    "file3",
			ModTime: time.Unix(0, 0),
		}, {
			Name:    "file4",
			ModTime: time.Unix(1400000000, 0),
		}},
	}, {
		input: "testdata/nil-uid.tar", // golang.org/issue/5290
		files: []file{{
			Name:    "P1050238.JPG.log",
			Mode:    0664,
			Size:    14,
			ModTime: time.Unix(1365454838, 0),
		}},
	}, {
		input: "testdata/xattrs.tar",
		files: []file{{
			Name:       "small.txt",
			Mode:       0644,
			Uid:        1000,
			Gid:        10,
			Size:       5,
			ModTime:    time.Unix(1386065770, 448252320),
			AccessTime: time.Unix(1389782991, 419875220),
			ChangeTime: time.Unix(1389782956, 794414986),
			Xattrs: map[string]string{
				"user.key":  "value",
				"user.key2": "value2",
				// Interestingly, selinux encodes the terminating null inside the xattr
				"security.selinux": "unconfined_u:object_r:default_t:s0\x00",
			},
		}, {
			Name:       "small2.txt",
			Mode:       0644,
			Uid:        1000,
			Gid:        10,
			Size:       11,
			ModTime:    time.Unix(1386065770, 449252304),
			AccessTime: time.Unix(1389782991, 419875220),
			ChangeTime: time.Unix(1386065770, 449252304),
			Xattrs: map[string]string{
				"security.selinux": "unconfined_u:object_r:default_t:s0\x00",
			},
		}},
	}, {
		// Matches the behavior of GNU, BSD, and STAR tar utilities.
		input: "testdata/gnu-multi-hdrs.tar",
		files: []file{{
			Name:      "GNU2/GNU2/long-path-name",
			Linkname:  "GNU4/GNU4/long-linkpath-name",
			ModTime:   time.Unix(0, 0),
			IsSymlink: true,
		}},
	}, {
		// Matches the behavior of GNU and BSD tar utilities.
		input: "testdata/pax-multi-hdrs.tar",
		files: []file{{
			Name:      "bar",
			Linkname:  "PAX4/PAX4/long-linkpath-name",
			ModTime:   time.Unix(0, 0),
			IsSymlink: true,
		}},
	}, {
		// Both BSD and GNU tar truncate long names at first NUL even
		// if there is data following that NUL character.
		// This is reasonable as GNU long names are C-strings.
		input: "testdata/gnu-long-nul.tar",
		files: []file{{
			Name:    "0123456789",
			Mode:    0644,
			Uid:     1000,
			Gid:     1000,
			ModTime: time.Unix(1486082191, 0),
		}},
	}, {
		// This archive was generated by Writer but is readable by both
		// GNU and BSD tar utilities.
		// The archive generated by GNU is nearly byte-for-byte identical
		// to the Go version except the Go version sets a negative Devminor
		// just to force the GNU format.
		input: "testdata/gnu-utf8.tar",
		files: []file{{
			Name:    "☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹☺☻☹",
			Mode:    0644,
			Uid:     1000,
			Gid:     1000,
			ModTime: time.Unix(0, 0),
		}},
	}, {
		// This archive was generated by Writer but is readable by both
		// GNU and BSD tar utilities.
		// The archive generated by GNU is nearly byte-for-byte identical
		// to the Go version except the Go version sets a negative Devminor
		// just to force the GNU format.
		input: "testdata/gnu-not-utf8.tar",
		files: []file{{
			Name:    "hi\x80\x81\x82\x83bye",
			Mode:    0644,
			Uid:     1000,
			Gid:     1000,
			ModTime: time.Unix(0, 0),
		}},
	}, {
		// USTAR archive with a regular entry with non-zero device numbers.
		input: "testdata/ustar-file-devs.tar",
		files: []file{{
			Name:     "file",
			Mode:     0644,
			ModTime:  time.Unix(0, 0),
			DevMajor: 1,
			DevMinor: 1,
		}},
	}, {
		// Generated by Go, works on BSD tar v3.1.2 and GNU tar v.1.27.1.
		input: "testdata/gnu-nil-sparse-data.tar",
		files: []file{{
			Name:    "sparse.db",
			Size:    1000,
			ModTime: time.Unix(0, 0),
		}},
	}, {
		// Generated by Go, works on BSD tar v3.1.2 and GNU tar v.1.27.1.
		input: "testdata/gnu-nil-sparse-hole.tar",
		files: []file{{
			Name:    "sparse.db",
			Size:    1000,
			ModTime: time.Unix(0, 0),
		}},
	}, {
		// Generated by Go, works on BSD tar v3.1.2 and GNU tar v.1.27.1.
		input: "testdata/pax-nil-sparse-data.tar",
		files: []file{{
			Name:    "sparse.db",
			Size:    1000,
			ModTime: time.Unix(0, 0),
		}},
	}, {
		// Generated by Go, works on BSD tar v3.1.2 and GNU tar v.1.27.1.
		input: "testdata/pax-nil-sparse-hole.tar",
		files: []file{{
			Name:    "sparse.db",
			Size:    1000,
			ModTime: time.Unix(0, 0),
		}},
	}, {
		input: "testdata/trailing-slash.tar",
		files: []file{{
			Name:    strings.Repeat("123456789/", 30),
			ModTime: time.Unix(0, 0),
			IsDir:   true,
		}},
	}}

	for _, v := range vectors {
		t.Run(path.Base(v.input), func(t *testing.T) {
			inputFile, err := os.Open(v.input)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, inputFile.Close())
			})

			fsys, err := tarfs.Open(inputFile)
			require.NoError(t, err)

			for _, file := range v.files {

				var fi fs.FileInfo
				var sum string
				if !file.IsSymlink {
					f, err := fsys.Open(file.Name)
					require.NoError(t, err, file.Name)

					h := md5.New()
					_, err = io.Copy(h, f)
					require.NoError(t, err)
					sum = fmt.Sprintf("%x", h.Sum(nil))

					fi, err = f.Stat()
					require.NoError(t, err)
				} else {
					fi, err = fsys.StatLink(file.Name)
					require.NoError(t, err)
				}

				require.Equal(t, filepath.Base(file.Name), fi.Name())
				require.Equal(t, file.Size, fi.Size())
				require.Equal(t, file.Mode, fi.Mode()&fs.ModePerm)
				require.Equal(t, file.ModTime, fi.ModTime())
				require.Equal(t, file.IsDir, fi.IsDir())

				stat, ok := fi.Sys().(*tar.Header)
				require.True(t, ok)

				require.Equal(t, file.Mode, fs.FileMode(stat.Mode)&fs.ModePerm)
				require.Equal(t, file.Uid, stat.Uid)
				require.Equal(t, file.Gid, stat.Gid)
				require.Equal(t, file.ModTime.Unix(), stat.ModTime.Unix())
				require.Equal(t, file.AccessTime.Unix(), stat.AccessTime.Unix())
				require.Equal(t, file.ChangeTime.Unix(), stat.ChangeTime.Unix())
				require.Equal(t, file.DevMajor, stat.Devmajor)
				require.Equal(t, file.DevMinor, stat.Devminor)
				if len(file.Xattrs) > 0 {
					for key, value := range file.Xattrs {
						require.Equal(t, value, stat.PAXRecords["SCHILY.xattr."+key])
					}
				}
				if file.Linkname != "" {
					require.Equal(t, file.Linkname, stat.Linkname)
				}

				if file.Sum != "" {
					require.Equal(t, file.Sum, sum)
				}
			}
		})
	}
}

func TestTarFSDirHash(t *testing.T) {
	f, err := os.Open("testdata/toybox.tar")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	fsys, err := tarfs.Open(f)
	require.NoError(t, err)

	var files []string
	err = fs.WalkDir(fsys, ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		files = append(files, filepath.ToSlash(file))
		return nil
	})
	require.NoError(t, err)

	h, err := dirhash.Hash1(files, func(name string) (io.ReadCloser, error) {
		return fsys.Open(name)
	})
	require.NoError(t, err)

	require.Equal(t, "h1:adgxkqVceeKMyJdMZMvcUIbg94TthnXUmOeufCPuzQI=", h)
}

func TestTarFSReadlink(t *testing.T) {
	f, err := os.Open("testdata/toybox.tar")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	fsys, err := tarfs.Open(f)
	require.NoError(t, err)

	foo, err := fsys.ReadLink("bin")
	require.NoError(t, err)

	require.Equal(t, "usr/bin", foo)
}

func TestTarFSStat(t *testing.T) {
	f, err := os.Open("testdata/toybox.tar")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	fsys, err := tarfs.Open(f)
	require.NoError(t, err)

	t.Run("Stat", func(t *testing.T) {
		fi, err := fsys.Stat("bin")
		require.NoError(t, err)

		require.True(t, fi.IsDir())
		require.Equal(t, "bin", fi.Name())
		require.Equal(t, fs.ModeDir|0o755, fi.Mode())
	})

	t.Run("StatLink", func(t *testing.T) {
		fi, err := fsys.StatLink("bin")
		require.NoError(t, err)

		require.False(t, fi.IsDir())
		require.Equal(t, "bin", fi.Name())
		require.Equal(t, fs.ModeSymlink|0o777, fi.Mode())
	})
}
