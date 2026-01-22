package buildtool_test

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/toejough/targ/buildtool"
)

func TestDiscover_AllowsContextFunctions(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

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

	if names := commandNamesByKind(infos[0], buildtool.CommandFunc); len(names) != 1 ||
		names[0] != runTestCmdName {
		t.Fatalf("unexpected funcs: %v", names)
	}
}

func TestDiscover_AllowsErrorReturnFunctions(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

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

	if names := commandNamesByKind(infos[0], buildtool.CommandFunc); len(names) != 1 ||
		names[0] != "Fail" {
		t.Fatalf("unexpected funcs: %v", names)
	}
}

func TestDiscover_ContextAliasImport(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

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

	if names := commandNamesByKind(infos[0], buildtool.CommandFunc); len(names) != 1 ||
		names[0] != runTestCmdName {
		t.Fatalf(
			"expected %s func to be detected with aliased context, got %v",
			runTestCmdName,
			names,
		)
	}
}

func TestDiscover_ContextImportVariations(t *testing.T) {
	tests := []struct {
		name       string
		importLine string
		funcLine   string
		expectFunc string
		comment    string
	}{
		{
			name:       "blank import",
			importLine: `import _ "context"`,
			funcLine:   "func Run() {}",
			expectFunc: "Run",
			comment:    "// Run without context param because blank import doesn't allow usage",
		},
		{
			name:       "dot import",
			importLine: `import . "context"`,
			funcLine:   "func Run(c Context) {}",
			expectFunc: "Run",
			comment:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			commentSection := ""
			if tc.comment != "" {
				commentSection = tc.comment + "\n"
			}

			fsMock := MockFileSystem(t)
			infos := runDiscoverTest(t, fsMock, []byte(`//go:build targ

package build

`+tc.importLine+`

`+commentSection+tc.funcLine+`
`))

			names := commandNamesByKind(infos[0], buildtool.CommandFunc)
			if len(names) != 1 || names[0] != tc.expectFunc {
				t.Fatalf("expected %s func to be detected, got %v", tc.expectFunc, names)
			}
		})
	}
}

func TestDiscover_DescriptionMethodVariations(t *testing.T) {
	tests := []struct {
		name            string
		descriptionFunc string
		expectedDesc    string
		failureReason   string
	}{
		{
			name:            "valid description",
			descriptionFunc: `func (b *Build) Description() string { return "Build the project" }`,
			expectedDesc:    "Build the project",
			failureReason:   "",
		},
		{
			name:            "multiple returns",
			descriptionFunc: `func (b *Build) Description() (string, error) { return "Build", nil }`,
			expectedDesc:    "",
			failureReason:   "multiple returns",
		},
		{
			name:            "with params",
			descriptionFunc: `func (b *Build) Description(locale string) string { return "Build the project" }`,
			expectedDesc:    "",
			failureReason:   "method has params",
		},
		{
			name:            "wrong return type",
			descriptionFunc: `func (b *Build) Description() int { return 42 }`,
			expectedDesc:    "",
			failureReason:   "wrong return type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fsMock := MockFileSystem(t)
			infos := runDiscoverTest(t, fsMock, []byte(`//go:build targ

package build

type Build struct{}

func (b *Build) Run() {}
`+tc.descriptionFunc+`
`))

			desc := findCommandDesc(infos[0], buildTestCmdName, buildtool.CommandStruct)

			if desc != tc.expectedDesc {
				if tc.failureReason != "" {
					t.Fatalf("expected no description (%s), got %q", tc.failureReason, desc)
				} else {
					t.Fatalf("expected description %q, got %q", tc.expectedDesc, desc)
				}
			}
		})
	}
}

func TestDiscover_DuplicateCommandNames(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.Discover(fsMock.Mock, opts)

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
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

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
	if names := commandNamesByKind(info, buildtool.CommandStruct); len(names) != 1 ||
		names[0] != "Root" {
		t.Fatalf("expected structs [Root], got %v", names)
	}
}

func TestDiscover_FiltersSubcommandsAndFuncs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

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

	if names := commandNamesByKind(info, buildtool.CommandStruct); len(names) != 1 ||
		names[0] != "Root" {
		t.Fatalf("expected structs [Root], got %v", names)
	}

	if names := commandNamesByKind(info, buildtool.CommandFunc); len(names) != 1 ||
		names[0] != buildTestCmdName {
		t.Fatalf("expected funcs [%s], got %v", buildTestCmdName, names)
	}
}

func TestDiscover_FunctionDocComment(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

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
		if cmd.Name == buildTestCmdName && cmd.Kind == buildtool.CommandFunc {
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
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

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
		if cmd.Name == "NoDocs" && cmd.Kind == buildtool.CommandFunc {
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
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

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
	if names := commandNamesByKind(info, buildtool.CommandStruct); len(names) != 0 {
		t.Fatalf("expected no structs (generated file skipped), got %v", names)
	}

	if names := commandNamesByKind(info, buildtool.CommandFunc); len(names) != 1 ||
		names[0] != buildTestCmdName {
		t.Fatalf("expected funcs [%s], got %v", buildTestCmdName, names)
	}
}

func TestDiscover_MultiplePackageNamesError(t *testing.T) {
	// Two files with different package names in the same directory should error
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.Discover(fsMock.Mock, opts)

		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd1.go", dir: false},
		fakeDirEntry{name: "cmd2.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd1.go").
		InjectReturnValues([]byte(`//go:build targ

package foo

func Run() {}
`), nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd2.go").
		InjectReturnValues([]byte(`//go:build targ

package bar

func Build() {}
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected multiple package names error")
	}

	if !strings.Contains(err.Error(), "multiple package names") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscover_PackageDoc(t *testing.T) {
	// Package doc comment should be captured
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

// Package build provides build commands.
package build

func Run() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}

	if infos[0].Doc != "Package build provides build commands." {
		t.Fatalf("expected package doc, got %q", infos[0].Doc)
	}
}

func TestDiscover_RejectsMainFunction(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.Discover(fsMock.Mock, opts)

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
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.Discover(fsMock.Mock, opts)

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
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.Discover(fsMock.Mock, opts)

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

	if errors.Is(err, buildtool.ErrNoTaggedFiles) {
		t.Fatalf("unexpected buildtool.ErrNoTaggedFiles: %v", err)
	}
}

func TestDiscover_RemovesWrappedFuncs(t *testing.T) {
	// When a struct named BuildCommand exists, function Build should be removed
	fsMock := MockFileSystem(t)
	infos := runDiscoverTest(t, fsMock, []byte(`//go:build targ

package build

// BuildCommand wraps Build function.
type BuildCommand struct{}

func (c *BuildCommand) Run() {}

// Build does something.
func Build() {}

// Deploy does something else.
func Deploy() {}
`))

	// Build should be removed (wrapped by BuildCommand)
	funcNames := commandNamesByKind(infos[0], buildtool.CommandFunc)
	for _, name := range funcNames {
		if name == "Build" {
			t.Fatal("Build function should have been removed (wrapped by BuildCommand)")
		}
	}

	// Deploy should remain (not wrapped)
	if !slices.Contains(funcNames, "Deploy") {
		t.Fatal("Deploy function should remain in funcList")
	}

	// BuildCommand struct should be in structList
	structNames := commandNamesByKind(infos[0], buildtool.CommandStruct)
	if !slices.Contains(structNames, "BuildCommand") {
		t.Fatalf("BuildCommand struct should be in structList, got: %v", structNames)
	}
}

func TestDiscover_ReturnsAllTaggedDirs(t *testing.T) {
	testMultipleTaggedPackages(
		t,
		func(fsMock *FileSystemMockHandle, opts buildtool.Options) (int, error) {
			infos, err := buildtool.Discover(fsMock.Mock, opts)
			return len(infos), err
		},
	)
}

func TestDiscover_SkipsTargCacheDir(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

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

// --- OSFileSystem tests ---

func TestOSFileSystem_ReadDir(t *testing.T) {
	dir := t.TempDir()

	fs := buildtool.OSFileSystem{}

	entries, err := fs.ReadDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("expected empty dir, got %d entries", len(entries))
	}
}

func TestOSFileSystem_ReadDir_Error(t *testing.T) {
	fs := buildtool.OSFileSystem{}

	_, err := fs.ReadDir("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestOSFileSystem_ReadFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.txt"
	expected := []byte("hello world")

	// Create file first using os package directly
	err := writeTestFile(path, expected)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	fs := buildtool.OSFileSystem{}

	content, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(content) != string(expected) {
		t.Fatalf("expected %q, got %q", expected, content)
	}
}

func TestOSFileSystem_ReadFile_Error(t *testing.T) {
	fs := buildtool.OSFileSystem{}

	_, err := fs.ReadFile("/nonexistent/file/that/does/not/exist.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestOSFileSystem_WriteFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/out.txt"
	expected := []byte("written content")

	fs := buildtool.OSFileSystem{}

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

func TestOSFileSystem_WriteFile_Error(t *testing.T) {
	fs := buildtool.OSFileSystem{}

	err := fs.WriteFile("/nonexistent/path/that/does/not/exist/file.txt", []byte("data"), 0o644)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestTaggedFiles_ReturnsSelectedFiles(t *testing.T) {
	testMultipleTaggedPackages(
		t,
		func(fsMock *FileSystemMockHandle, opts buildtool.Options) (int, error) {
			files, err := buildtool.TaggedFiles(fsMock.Mock, opts)
			return len(files), err
		},
	)
}

func TestTaggedFiles_SkipsUnreadableFiles(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		files []buildtool.TaggedFile
		err   error
	)

	go func() {
		files, err = buildtool.TaggedFiles(fsMock.Mock, opts)

		close(done)
	}()

	// Directory contains a file that fails to read (alphabetical order: readable before unreadable)
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "readable.go", dir: false},
		fakeDirEntry{name: "unreadable.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/readable.go").
		InjectReturnValues([]byte(`//go:build targ

package build

func Build() {}
`), nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/unreadable.go").
		InjectReturnValues([]byte(nil), errors.New("permission denied"))

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only return the readable file
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if files[0].Path != "/root/readable.go" {
		t.Fatalf("expected readable.go, got %s", files[0].Path)
	}
}

func TestDiscover_ExplicitRegistration(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/targets.go").
		InjectReturnValues([]byte(`//go:build targ

package dev

import "github.com/toejough/targ"

var Build = targ.Targ(build)

func build() {}

func init() {
	targ.Register(Build)
}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}

	if !infos[0].UsesExplicitRegistration {
		t.Fatal("expected UsesExplicitRegistration to be true")
	}
}

func TestDiscover_ExplicitRegistrationWithAlias(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/targets.go").
		InjectReturnValues([]byte(`//go:build targ

package dev

import t "github.com/toejough/targ"

var Build = t.Targ(build)

func build() {}

func init() {
	t.Register(Build)
}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}

	if !infos[0].UsesExplicitRegistration {
		t.Fatal("expected UsesExplicitRegistration to be true")
	}
}

func TestDiscover_NoExplicitRegistration(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "targets.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/targets.go").
		InjectReturnValues([]byte(`//go:build targ

package dev

func Build() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}

	if infos[0].UsesExplicitRegistration {
		t.Fatal("expected UsesExplicitRegistration to be false")
	}
}

// unexported constants.
const (
	buildTestCmdName = "Build"
	runTestCmdName   = "Run"
)

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

func commandNamesByKind(info buildtool.PackageInfo, kind buildtool.CommandKind) []string {
	var names []string

	for _, cmd := range info.Commands {
		if cmd.Kind == kind {
			names = append(names, cmd.Name)
		}
	}

	return names
}

// findCommandDesc finds the description of a command by name and kind.
func findCommandDesc(info buildtool.PackageInfo, name string, kind buildtool.CommandKind) string {
	for _, cmd := range info.Commands {
		if cmd.Name == name && cmd.Kind == kind {
			return cmd.Description
		}
	}

	return ""
}

// runDiscoverTest runs Discover and expects a single PackageInfo.
// Returns the infos for further assertions.
func runDiscoverTest(
	t *testing.T,
	fsMock *FileSystemMockHandle,
	fileContent []byte,
) []buildtool.PackageInfo {
	t.Helper()

	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		infos []buildtool.PackageInfo
		err   error
	)

	go func() {
		infos, err = buildtool.Discover(fsMock.Mock, opts)

		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues(fileContent, nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package info, got %d", len(infos))
	}

	return infos
}

func testMultipleTaggedPackages(
	t *testing.T,
	testFunc func(*FileSystemMockHandle, buildtool.Options) (int, error),
) {
	t.Helper()

	fsMock := MockFileSystem(t)
	opts := buildtool.Options{StartDir: "/root"}
	done := make(chan struct{})

	var (
		count int
		err   error
	)

	go func() {
		count, err = testFunc(fsMock, opts)

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

	if count != 2 {
		t.Fatalf("expected 2 items, got %d", count)
	}
}

func writeTestFile(path string, content []byte) error {
	return buildtool.OSFileSystem{}.WriteFile(path, content, 0o644)
}
