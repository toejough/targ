package buildtool

import (
	"errors"
	"go/ast"
	"io/fs"
	"strings"
	"testing"
	"time"
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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

func TestDiscover_ContextAliasImport(t *testing.T) {
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

import ctx "context"

func Run(c ctx.Context) {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}

	if names := commandNamesByKind(infos[0], CommandFunc); len(names) != 1 || names[0] != "Run" {
		t.Fatalf("expected Run func to be detected with aliased context, got %v", names)
	}
}

func TestDiscover_ContextBlankImport(t *testing.T) {
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

import _ "context"

// Run without context param because blank import doesn't allow usage
func Run() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}
	// Blank import should be ignored for context detection
	if names := commandNamesByKind(infos[0], CommandFunc); len(names) != 1 || names[0] != "Run" {
		t.Fatalf("expected Run func to be detected, got %v", names)
	}
}

func TestDiscover_ContextDotImport(t *testing.T) {
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

import . "context"

func Run(c Context) {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}

	if names := commandNamesByKind(infos[0], CommandFunc); len(names) != 1 || names[0] != "Run" {
		t.Fatalf("expected Run func to be detected with dot import context, got %v", names)
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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

func TestDiscover_DescriptionMultipleReturns(t *testing.T) {
	// Description method returning multiple values should be ignored
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

type Build struct{}

func (b *Build) Run() {}
func (b *Build) Description() (string, error) { return "Build", nil }
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
	// Description returning multiple values should be ignored
	if desc != "" {
		t.Fatalf("expected no description (multiple returns), got %q", desc)
	}
}

func TestDiscover_DescriptionWithParams(t *testing.T) {
	// Description method with params should be ignored
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

type Build struct{}

func (b *Build) Run() {}
func (b *Build) Description(locale string) string { return "Build the project" }
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
	// Description with params should be ignored, so no description
	if desc != "" {
		t.Fatalf("expected no description (method has params), got %q", desc)
	}
}

func TestDiscover_DescriptionWrongReturnType(t *testing.T) {
	// Description method returning int should be ignored
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

type Build struct{}

func (b *Build) Run() {}
func (b *Build) Description() int { return 42 }
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
	// Description returning int should be ignored
	if desc != "" {
		t.Fatalf("expected no description (wrong return type), got %q", desc)
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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

func TestDiscover_FunctionDocComment(t *testing.T) {
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

// Build compiles the project.
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

	var desc string

	for _, cmd := range info.Commands {
		if cmd.Name == "Build" && cmd.Kind == CommandFunc {
			desc = cmd.Description
			break
		}
	}

	if desc != "Build compiles the project." {
		t.Fatalf("expected doc comment description, got %q", desc)
	}
}

func TestDiscover_FunctionNoDocComment(t *testing.T) {
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

func NoDocs() {}
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
		if cmd.Name == "NoDocs" && cmd.Kind == CommandFunc {
			desc = cmd.Description
			break
		}
	}

	if desc != "" {
		t.Fatalf("expected empty description for function without doc, got %q", desc)
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/pkg1/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package pkg1

func Hello() {}
`), nil)
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root/pkg2").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/pkg2/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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

func TestHasBuildTag_CommentBeforeBuildTag(t *testing.T) {
	content := []byte(`// Some comment
//go:build targ

package main
`)
	if !hasBuildTag(content, "targ") {
		t.Fatal("expected match with comment before build tag")
	}
}

func TestHasBuildTag_EmptyContent(t *testing.T) {
	if hasBuildTag([]byte(""), "targ") {
		t.Fatal("expected no match for empty content")
	}
}

func TestHasBuildTag_EmptyLinesBeforeBuildTag(t *testing.T) {
	content := []byte(`

//go:build targ

package main
`)
	if !hasBuildTag(content, "targ") {
		t.Fatal("expected match with empty lines before build tag")
	}
}

func TestHasBuildTag_InvalidBuildConstraint(t *testing.T) {
	// Invalid constraint syntax - should fall back to string match
	content := []byte(`//go:build !!!invalid

package main
`)
	// When constraint parsing fails, it falls back to exact string match
	if !hasBuildTag(content, "!!!invalid") {
		t.Fatal("expected match for invalid constraint with exact string match")
	}

	if hasBuildTag(content, "targ") {
		t.Fatal("expected no match for valid tag when constraint is invalid")
	}
}

func TestHasBuildTag_NonCommentLineFirst(t *testing.T) {
	content := []byte(`package main

//go:build targ
`)
	if hasBuildTag(content, "targ") {
		t.Fatal("expected no match when non-comment line comes first")
	}
}

// --- OSFileSystem tests ---

func TestOSFileSystem_ReadDir(t *testing.T) {
	dir := t.TempDir()

	fs := OSFileSystem{}

	entries, err := fs.ReadDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("expected empty dir, got %d entries", len(entries))
	}
}

func TestOSFileSystem_ReadFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.txt"
	expected := []byte("hello world")

	// Create file first using os package directly
	if err := writeTestFile(path, expected); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	fs := OSFileSystem{}

	content, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(content) != string(expected) {
		t.Fatalf("expected %q, got %q", expected, content)
	}
}

func TestOSFileSystem_WriteFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/out.txt"
	expected := []byte("written content")

	fs := OSFileSystem{}

	err := fs.WriteFile(path, expected, 0o644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify by reading back
	content, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}

	if string(content) != string(expected) {
		t.Fatalf("expected %q, got %q", expected, content)
	}
}

func TestReceiverTypeName_EmptyList(t *testing.T) {
	// Defensive code path - empty field list
	recv := &ast.FieldList{List: []*ast.Field{}}

	result := receiverTypeName(recv)
	if result != "" {
		t.Fatalf("expected empty string for empty list, got %q", result)
	}
}

func TestReceiverTypeName_Nil(t *testing.T) {
	// Defensive code path - nil receiver
	result := receiverTypeName(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil receiver, got %q", result)
	}
}

func TestReflectTagGet_Found(t *testing.T) {
	tag := reflectTag(`json:"name" targ:"flag"`)
	if got := tag.Get("targ"); got != "flag" {
		t.Fatalf("expected 'flag', got '%s'", got)
	}
}

func TestReflectTagGet_NoColon(t *testing.T) {
	tag := reflectTag(`json`)
	if got := tag.Get("json"); got != "" {
		t.Fatalf("expected empty for malformed tag, got '%s'", got)
	}
}

func TestReflectTagGet_NoQuoteAfterColon(t *testing.T) {
	tag := reflectTag(`json:name`)
	if got := tag.Get("json"); got != "" {
		t.Fatalf("expected empty for missing quote, got '%s'", got)
	}
}

func TestReflectTagGet_NotFound(t *testing.T) {
	tag := reflectTag(`json:"name"`)
	if got := tag.Get("targ"); got != "" {
		t.Fatalf("expected empty, got '%s'", got)
	}
}

func TestReflectTagGet_UnclosedQuote(t *testing.T) {
	tag := reflectTag(`json:"name`)
	if got := tag.Get("json"); got != "" {
		t.Fatalf("expected empty for unclosed quote, got '%s'", got)
	}
}

func TestReturnStringLiteral_EmptyBody(t *testing.T) {
	body := &ast.BlockStmt{List: []ast.Stmt{}}

	result, ok := returnStringLiteral(body)
	if ok || result != "" {
		t.Fatalf("expected false/empty for empty body, got %q/%v", result, ok)
	}
}

func TestReturnStringLiteral_MultipleResults(t *testing.T) {
	body := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ReturnStmt{
				Results: []ast.Expr{
					&ast.Ident{Name: "a"},
					&ast.Ident{Name: "b"},
				},
			},
		},
	}

	result, ok := returnStringLiteral(body)
	if ok || result != "" {
		t.Fatalf("expected false/empty for multiple results, got %q/%v", result, ok)
	}
}

func TestReturnStringLiteral_NilBody(t *testing.T) {
	result, ok := returnStringLiteral(nil)
	if ok || result != "" {
		t.Fatalf("expected false/empty for nil body, got %q/%v", result, ok)
	}
}

func TestReturnStringLiteral_NotBasicLit(t *testing.T) {
	body := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ReturnStmt{
				Results: []ast.Expr{&ast.Ident{Name: "variable"}},
			},
		},
	}

	result, ok := returnStringLiteral(body)
	if ok || result != "" {
		t.Fatalf("expected false/empty for non-literal return, got %q/%v", result, ok)
	}
}

func TestReturnStringLiteral_NotReturnStmt(t *testing.T) {
	body := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ExprStmt{X: &ast.Ident{Name: "foo"}},
		},
	}

	result, ok := returnStringLiteral(body)
	if ok || result != "" {
		t.Fatalf("expected false/empty for non-return stmt, got %q/%v", result, ok)
	}
}

func TestShouldSkipGoFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"regular go file", "main.go", false},
		{"test file", "main_test.go", true},
		{"generated targ file", "generated_targ_bootstrap.go", true},
		{"non-go file", "readme.md", true},
		{"non-go file txt", "notes.txt", true},
		{"go in name but wrong suffix", "go.mod", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipGoFile(tt.filename)
			if got != tt.want {
				t.Errorf("shouldSkipGoFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
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
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/pkg1/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package pkg1
`), nil)
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root/pkg2").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/pkg2/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

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

func TestTryReadTaggedFile_ReadFileError(t *testing.T) {
	fsMock := MockFileSystem(t)
	done := make(chan struct{})

	var (
		result taggedFile
		ok     bool
	)

	go func() {
		result, ok = tryReadTaggedFile(fsMock.Mock, "/test/file.go", "file.go", "targ")

		close(done)
	}()

	fsMock.Method.ReadFile.ExpectCalledWithExactly("/test/file.go").
		InjectReturnValues([]byte(nil), errors.New("read error"))

	<-done

	if ok {
		t.Fatal("expected false, got true")
	}

	if result.Path != "" {
		t.Fatalf("expected empty path, got %q", result.Path)
	}
}

type fakeDirEntry struct {
	name string
	dir  bool
}

func (f fakeDirEntry) Info() (fs.FileInfo, error) { return fakeFileInfo(f), nil }

func (f fakeDirEntry) IsDir() bool { return f.dir }

func (f fakeDirEntry) Name() string { return f.name }

func (f fakeDirEntry) Type() fs.FileMode { return 0 }

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

func commandNamesByKind(info PackageInfo, kind CommandKind) []string {
	var names []string

	for _, cmd := range info.Commands {
		if cmd.Kind == kind {
			names = append(names, cmd.Name)
		}
	}

	return names
}

func writeTestFile(path string, content []byte) error {
	return OSFileSystem{}.WriteFile(path, content, 0o644)
}
