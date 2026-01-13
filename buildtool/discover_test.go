package buildtool

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestDiscover_AllowsContextFunctions(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

import "context"

func Run(ctx context.Context) {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}
	if names := commandNamesByKind(infos[0], CommandFunc); len(names) != 1 || names[0] != "Run" {
		t.Fatalf("unexpected funcs: %v", names)
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
		infos, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

func Fail() error { return nil }
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}
	if names := commandNamesByKind(infos[0], CommandFunc); len(names) != 1 || names[0] != "Fail" {
		t.Fatalf("unexpected funcs: %v", names)
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
		infos, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

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
	info := infos[0]
	var desc string
	for _, cmd := range info.Commands {
		if cmd.Name == "Build" && cmd.Kind == CommandStruct {
			desc = cmd.Description
			break
		}
	}
	if desc != "Build the project" {
		t.Fatalf("expected description, got %q", desc)
	}
}

func TestDiscover_DuplicateCommandNames(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var err error

	go func() {
		_, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

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

func TestDiscover_FiltersNonRunnableStructs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

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
	if names := commandNamesByKind(info, CommandStruct); len(names) != 1 || names[0] != "Root" {
		t.Fatalf("expected structs [Root], got %v", names)
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
		infos, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

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
	if names := commandNamesByKind(info, CommandStruct); len(names) != 1 || names[0] != "Root" {
		t.Fatalf("expected structs [Root], got %v", names)
	}
	if names := commandNamesByKind(info, CommandFunc); len(names) != 1 || names[0] != "Build" {
		t.Fatalf("expected funcs [Build], got %v", names)
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
		infos, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
		fakeDirEntry{name: "generated_targ_build.go", dir: false},
	}, nil)
	// generated_targ_* files should be skipped, not read
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

func Build() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}
	info := infos[0]
	// Only the function from cmd.go should be found, not the struct from generated file
	if names := commandNamesByKind(info, CommandStruct); len(names) != 0 {
		t.Fatalf("expected no structs (generated file skipped), got %v", names)
	}
	if names := commandNamesByKind(info, CommandFunc); len(names) != 1 || names[0] != "Build" {
		t.Fatalf("expected funcs [Build], got %v", names)
	}
}

func TestDiscover_RejectsMainFunction(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var err error

	go func() {
		_, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

func main() {}
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected error for tagged main function")
	}
	if !strings.Contains(err.Error(), "main()") || !strings.Contains(err.Error(), "/root/cmd.go") {
		t.Fatalf("expected error to mention main and file, got: %v", err)
	}
}

func TestDiscover_RejectsNonErrorReturnFunctions(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var err error

	go func() {
		_, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

func Bad() int { return 1 }
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected error for non-error return function")
	}
}

func TestDiscover_RejectsNonNiladicFunctions(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var err error

	go func() {
		_, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

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

func TestDiscover_ReturnsAllTaggedDirs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "pkg1", dir: true},
		fakeDirEntry{name: "pkg2", dir: true},
	}, nil)
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root/pkg1").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/pkg1/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg1

func Hello() {}
`), nil)
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root/pkg2").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/pkg2/cmd.go").InjectReturnValues([]byte(`//go:build targ

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

func TestDiscover_SkipsTargCacheDir(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		infos []PackageInfo
		err   error
	)

	go func() {
		infos, err = Discover(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: ".git", dir: true},
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").InjectReturnValues([]byte(`//go:build targ

package build

func Build() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
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

func TestTaggedFiles_ReturnsSelectedFiles(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		files []TaggedFile
		err   error
	)

	go func() {
		files, err = TaggedFiles(fsMock.Mock, opts)
		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "pkg1", dir: true},
		fakeDirEntry{name: "pkg2", dir: true},
	}, nil)
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root/pkg1").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/pkg1/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg1
`), nil)
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root/pkg2").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/pkg2/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg2
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 tagged files, got %d", len(files))
	}
}

type fakeDirEntry struct {
	name string
	dir  bool
}

func (f fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func (f fakeDirEntry) IsDir() bool { return f.dir }

func (f fakeDirEntry) Name() string { return f.name }

func (f fakeDirEntry) Type() fs.FileMode { return 0 }

func commandNamesByKind(info PackageInfo, kind CommandKind) []string {
	var names []string
	for _, cmd := range info.Commands {
		if cmd.Kind == kind {
			names = append(names, cmd.Name)
		}
	}
	return names
}
