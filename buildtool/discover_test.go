package buildtool_test

import (
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/toejough/targ/buildtool"
)

func TestDiscover_BasicPackage(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package dev

import "github.com/toejough/targ"

func init() {
	targ.Register(Build)
}

var Build = targ.Targ(build)
func build() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}

	if infos[0].Package != "dev" {
		t.Errorf("expected package 'dev', got %q", infos[0].Package)
	}

	if !infos[0].UsesExplicitRegistration {
		t.Error("expected UsesExplicitRegistration to be true")
	}

	if len(infos[0].Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(infos[0].Files))
	}
}

func TestDiscover_CapturesPackageDoc(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

// Package dev provides development targets.
package dev

import "github.com/toejough/targ"

func init() { targ.Register() }
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) == 0 {
		t.Fatal("expected at least 1 package info")
	}

	if infos[0].Doc != "Package dev provides development targets." {
		t.Errorf("unexpected doc: %q", infos[0].Doc)
	}
}

func TestDiscover_DetectsExplicitRegistration(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package dev

import "github.com/toejough/targ"

func init() {
	targ.Register(MyTarget)
}

var MyTarget = targ.Targ(myTarget)
func myTarget() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) == 0 {
		t.Fatal("expected at least 1 package info")
	}

	if !infos[0].UsesExplicitRegistration {
		t.Error("expected UsesExplicitRegistration to be true")
	}
}

func TestDiscover_DetectsExplicitRegistrationWithAlias(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package dev

import t "github.com/toejough/targ"

func init() {
	t.Register(MyTarget)
}

var MyTarget = t.Targ(myTarget)
func myTarget() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) == 0 {
		t.Fatal("expected at least 1 package info")
	}

	if !infos[0].UsesExplicitRegistration {
		t.Error("expected UsesExplicitRegistration to be true with alias")
	}
}

func TestDiscover_ErrorsOnMainFunction(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package dev

func main() {}
`), nil)

	<-done

	if !errors.Is(err, buildtool.ErrMainFunctionNotAllowed) {
		t.Errorf("expected ErrMainFunctionNotAllowed, got %v", err)
	}
}

func TestDiscover_ErrorsOnMultiplePackageNames(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "a.go", dir: false},
		fakeDirEntry{name: "b.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/a.go").
		Return([]byte(`//go:build targ

package dev
`), nil)
	imp.ReadFile.ArgsEqual("/root/b.go").
		Return([]byte(`//go:build targ

package other
`), nil)

	<-done

	if !errors.Is(err, buildtool.ErrMultiplePackageNames) {
		t.Errorf("expected ErrMultiplePackageNames, got %v", err)
	}
}

func TestDiscover_MultiplePackages(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "dev", dir: true},
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package root

import "github.com/toejough/targ"

func init() { targ.Register() }
`), nil)
	imp.ReadDir.ArgsEqual("/root/dev").Return([]fs.DirEntry{
		fakeDirEntry{name: "dev.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/dev/dev.go").
		Return([]byte(`//go:build targ

package dev

import "github.com/toejough/targ"

func init() { targ.Register() }
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 2 {
		t.Errorf("expected 2 packages, got %d", len(infos))
	}
}

func TestDiscover_NoExplicitRegistration(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package dev

func Build() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) == 0 {
		t.Fatal("expected at least 1 package info")
	}

	if infos[0].UsesExplicitRegistration {
		t.Error("expected UsesExplicitRegistration to be false")
	}
}

func TestDiscover_NoTaggedFiles(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "main.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/main.go").
		Return([]byte(`package main

func main() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 0 {
		t.Errorf("expected 0 packages, got %d", len(infos))
	}
}

func TestDiscover_SkipsHiddenDirectories(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: ".hidden", dir: true},
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package dev

import "github.com/toejough/targ"

func init() { targ.Register() }
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have 1 package (the root), not the hidden dir
	if len(infos) != 1 {
		t.Errorf("expected 1 package, got %d", len(infos))
	}
}

func TestDiscover_SkipsTestFiles(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
		fakeDirEntry{name: "targets_test.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package dev

import "github.com/toejough/targ"

func init() { targ.Register() }
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) == 0 {
		t.Fatal("expected at least 1 package info")
	}

	// Should only have 1 file (targets.go), not the test file
	if len(infos[0].Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(infos[0].Files))
	}
}

func TestDiscover_SkipsVendorDirectory(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "vendor", dir: true},
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package dev

import "github.com/toejough/targ"

func init() { targ.Register() }
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Errorf("expected 1 package, got %d", len(infos))
	}
}

func TestSelectTaggedDirs_ReturnsDirectories(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		dirs []buildtool.TaggedDir
		err  error
	)

	go func() {
		dirs, err = buildtool.SelectTaggedDirs(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package dev
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir, got %d", len(dirs))
	}

	if dirs[0].Path != "/root" {
		t.Errorf("expected path '/root', got %q", dirs[0].Path)
	}
}

func TestTaggedFiles_ReturnsFiles(t *testing.T) {
	mock, imp := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		files []buildtool.TaggedFile
		err   error
	)

	go func() {
		files, err = buildtool.TaggedFiles(mock, opts)

		close(done)
	}()

	imp.ReadDir.ArgsEqual("/root").Return([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	imp.ReadFile.ArgsEqual("/root/targets.go").
		Return([]byte(`//go:build targ

package dev
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if files[0].Path != "/root/targets.go" {
		t.Errorf("expected path '/root/targets.go', got %q", files[0].Path)
	}
}

type fakeDirEntry struct {
	name string
	dir  bool
}

func (f fakeDirEntry) Info() (fs.FileInfo, error) {
	return fakeFileInfo(f), nil
}

func (f fakeDirEntry) IsDir() bool { return f.dir }

func (f fakeDirEntry) Name() string { return f.name }

func (f fakeDirEntry) Type() fs.FileMode {
	if f.dir {
		return fs.ModeDir
	}

	return 0
}

type fakeFileInfo struct {
	name string
	dir  bool
}

func (f fakeFileInfo) IsDir() bool { return f.dir }

func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }

func (f fakeFileInfo) Mode() fs.FileMode { return 0 }

func (f fakeFileInfo) Name() string { return f.name }

func (f fakeFileInfo) Size() int64 { return 0 }

func (f fakeFileInfo) Sys() any { return nil }
