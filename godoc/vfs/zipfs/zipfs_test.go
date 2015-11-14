// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.package zipfs
package zipfs

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"golang.org/x/tools/godoc/vfs"
)

var (

	// files to use to build zip used by zipfs in testing; maps path : contents
	files = map[string]string{"foo": "foo", "bar/baz": "baz"}

	// expected info for each entry in a file system described by files
	tests = []struct {
		Path      string
		IsDir     bool
		IsRegular bool
		Name      string
		Contents  string
		Files     map[string]bool
	}{
		{"/", true, false, "", "", map[string]bool{"foo": true, "bar": true}},
		{"//", true, false, "", "", map[string]bool{"foo": true, "bar": true}},
		{"/foo", false, true, "foo", "foo", nil},
		{"/foo/", false, true, "foo", "foo", nil},
		{"/foo//", false, true, "foo", "foo", nil},
		{"/bar", true, false, "bar", "", map[string]bool{"baz": true}},
		{"/bar/", true, false, "bar", "", map[string]bool{"baz": true}},
		{"/bar/baz", false, true, "baz", "baz", nil},
		{"//bar//baz", false, true, "baz", "baz", nil},
	}

	// to be initialized in setup()
	fs        vfs.FileSystem
	statFuncs []statFunc
)

type statFunc struct {
	Name string
	Func func(string) (os.FileInfo, error)
}

func TestMain(t *testing.M) {
	if err := setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up zipfs testing state: %v.\n", err)
		os.Exit(1)
	}
	os.Exit(t.Run())
}

// setups state each of the tests uses
func setup() error {
	// create zipfs
	b := new(bytes.Buffer)
	zw := zip.NewWriter(b)
	for file, contents := range files {
		w, err := zw.Create(file)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, contents)
		if err != nil {
			return err
		}
	}
	zw.Close()
	zr, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err != nil {
		return err
	}
	rc := &zip.ReadCloser{
		Reader: *zr,
	}
	fs = New(rc, "foo")

	// pull out different stat functions
	statFuncs = []statFunc{
		{"Stat", fs.Stat},
		{"Lstat", fs.Lstat},
	}

	return nil
}

func TestZipFSReadDir(t *testing.T) {
	for _, test := range tests {
		if test.IsDir {
			infos, err := fs.ReadDir(test.Path)
			if err != nil {
				t.Errorf("Failed to read directory %v\n", test.Path)
				continue
			}
			actualFiles := make(map[string]bool)
			for _, info := range infos {
				actualFiles[info.Name()] = true
			}
			if eq := reflect.DeepEqual(test.Files, actualFiles); !eq {
				t.Errorf("Dir %v Expected Files != Actual Files\nExpected: %v\nActual: %v\n", test.Path, test.Files, actualFiles)
			}
		}
	}
}

func TestZipFSStatFuncs(t *testing.T) {
	for _, test := range tests {
		for _, statFunc := range statFuncs {

			// test can stat
			info, err := statFunc.Func(test.Path)
			if err != nil {
				t.Errorf("Unexpected error using %v for %v: %v\n", statFunc.Name, test.Path, err)
				continue
			}

			// test info.Name()
			if actualName := info.Name(); test.Name != actualName {
				t.Errorf("Using %v for %v name expected to be %v was %v\n", statFunc.Name, test.Path, test.Name, actualName)
			}
			// test info.IsDir()
			if actualIsDir := info.IsDir(); test.IsDir != actualIsDir {
				t.Errorf("Using %v for %v info.IsDir() returned %v\n", statFunc.Name, test.Path, actualIsDir)
			}
			// test info.Mode().IsDir()
			if actualIsDir := info.Mode().IsDir(); test.IsDir != actualIsDir {
				t.Errorf("Using %v for %v info.Mode().IsDir() returned %v\n", statFunc.Name, test.Path, actualIsDir)
			}
			// test info.Mode().IsRegular()
			if actualIsRegular := info.Mode().IsRegular(); test.IsRegular != actualIsRegular {
				t.Errorf("Using %v for %v info.Mode().IsRegular() returned %v\n", statFunc.Name, test.Path, actualIsRegular)
			}
			// test info.Size()
			if test.IsRegular {
				if actualSize := info.Size(); int64(len(test.Contents)) != actualSize {
					t.Errorf("Using %v for %v expected size %v was %v", statFunc.Name, test.Path, len(test.Contents), actualSize)
				}
			}
		}
	}
}

func TestZipFSOpenSeek(t *testing.T) {
	for _, test := range tests {
		if test.IsRegular {

			// test Open()
			f, err := fs.Open(test.Path)
			if err != nil {
				t.Error(err)
				return
			}
			defer f.Close()

			// test Seek() multiple times
			for i := 0; i < 3; i++ {
				all, err := ioutil.ReadAll(f)
				if err != nil {
					t.Error(err)
					return
				}
				actualContents := string(all)
				if test.Contents != actualContents {
					t.Errorf("File %v Contents %v != Actual %v", test.Path, test.Contents, actualContents)
				}
				f.Seek(0, 0)
			}
		}
	}
}
