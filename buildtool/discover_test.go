package buildtool

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

type fakeDirEntry struct {
	name string
	dir  bool
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return f.dir }
func (f fakeDirEntry) Type() fs.FileMode          { return 0 }
func (f fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func TestDiscover_DepthGatingErrorsOnMultipleDirs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var err error

	go func() {
		_, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "pkg1", dir: true},
		fakeDirEntry{name: "pkg2", dir: true},
	}, nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg1").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg1/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg1

func Hello() {}
`), nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg2").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg2/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg2

func Hi() {}
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected error for multiple tagged dirs at same depth")
	}
	if !strings.Contains(err.Error(), "/root/pkg1") || !strings.Contains(err.Error(), "/root/pkg2") {
		t.Fatalf("expected error to list conflicting paths, got: %v", err)
	}
}

func TestDiscover_FiltersSubcommandsAndFuncs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

type Root struct {
	Sub *SubCmd `+"`targ:\"subcommand\"`"+`
}

type SubCmd struct {}

func Build() {}
func Sub() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}
	info := infos[0]
	if info.Package != "build" {
		t.Fatalf("expected package name build, got %q", info.Package)
	}
	if len(info.Structs) != 1 || info.Structs[0] != "Root" {
		t.Fatalf("expected structs [Root], got %v", info.Structs)
	}
	if len(info.Funcs) != 1 || info.Funcs[0] != "Build" {
		t.Fatalf("expected funcs [Build], got %v", info.Funcs)
	}
}

func TestDiscover_DescriptionMethod(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

type Build struct{}

func (b *Build) Run() {}
func (b *Build) Description() string { return "Build the project" }
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}
	desc := infos[0].StructDescriptions["Build"]
	if desc != "Build the project" {
		t.Fatalf("expected description, got %q", desc)
	}
}
func TestDiscover_FiltersNonRunnableStructs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

type Root struct {
	Sub *SubCmd `+"`targ:\"subcommand\"`"+`
}

type Helper struct{}
type SubCmd struct{}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}
	info := infos[0]
	if len(info.Structs) != 1 || info.Structs[0] != "Root" {
		t.Fatalf("expected structs [Root], got %v", info.Structs)
	}
}

func TestTaggedFiles_ReturnsSelectedFiles(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		files []TaggedFile
		err   error
	)

	go func() {
		files, err = TaggedFiles(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "pkg1", dir: true},
		fakeDirEntry{name: "pkg2", dir: true},
	}, nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg1").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg1/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg1
`), nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg2").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg2/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg2
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected error for multiple tagged dirs at same depth")
	}
	if files != nil {
		t.Fatalf("expected no files on error, got %v", files)
	}
}

func TestDiscover_FunctionWrappersOverrideFuncs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
		fakeDirEntry{name: "generated_targ_build.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

func Build() {}
`), nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/generated_targ_build.go").InjectReturnValues([]byte(`//go:build targ

package build

type BuildCommand struct{}

func (c *BuildCommand) Run() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}
	info := infos[0]
	if len(info.Structs) != 1 || info.Structs[0] != "BuildCommand" {
		t.Fatalf("expected structs [BuildCommand], got %v", info.Structs)
	}
	if len(info.Funcs) != 0 {
		t.Fatalf("expected funcs to be empty, got %v", info.Funcs)
	}
}

func TestDiscover_RejectsNonNiladicFunctions(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var err error

	go func() {
		_, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

func Bad(a int) {}
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected error for non-niladic function")
	}
	if !strings.Contains(err.Error(), "Bad") {
		t.Fatalf("expected error to mention function name, got: %v", err)
	}
	if errors.Is(err, ErrNoTaggedFiles) {
		t.Fatalf("unexpected ErrNoTaggedFiles: %v", err)
	}
}

func TestHasBuildTag(t *testing.T) {
	content := []byte(`//go:build targ

package main
`)
	if !hasBuildTag(content, "targ") {
		t.Fatal("expected build tag to match")
	}
	if hasBuildTag(content, "other") {
		t.Fatal("expected build tag not to match other")
	}
}

func TestDiscover_AllowsErrorReturnFunctions(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

func Fail() error { return nil }
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 || len(infos[0].Funcs) != 1 || infos[0].Funcs[0] != "Fail" {
		t.Fatalf("unexpected funcs: %v", infos)
	}
}

func TestDiscover_AllowsContextFunctions(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

import "context"

func Run(ctx context.Context) {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 || len(infos[0].Funcs) != 1 || infos[0].Funcs[0] != "Run" {
		t.Fatalf("unexpected funcs: %v", infos)
	}
}

func TestDiscover_RejectsNonErrorReturnFunctions(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var err error

	go func() {
		_, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

func Bad() int { return 1 }
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected error for non-error return function")
	}
}

func TestDiscover_MultiPackageAllowsMultipleDirs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root", MultiPackage: true}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "pkg1", dir: true},
		fakeDirEntry{name: "pkg2", dir: true},
	}, nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg1").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg1/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg1

func Hello() {}
`), nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg2").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg2/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg2

func Hi() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 package infos, got %d", len(infos))
	}
}

func TestDiscover_DuplicateCommandNames(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var err error

	go func() {
		_, err = Discover(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

type FooBar struct{}
func (f *FooBar) Run() {}
func FooBar() {}
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected duplicate command name error")
	}
}
