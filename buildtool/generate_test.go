package buildtool_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/toejough/targ/buildtool"
)

func TestGenerateFunctionWrappers_BuildTagAndDescriptions(t *testing.T) {
	fsMock := MockFileSystem(t)
	done := make(chan struct{})

	var (
		path string
		err  error
	)

	go func() {
		path, err = buildtool.GenerateFunctionWrappers(fsMock.Mock, buildtool.GenerateOptions{
			Dir:        "/root",
			BuildTag:   "targ",
			OnlyTagged: true,
		})

		close(done)
	}()

	setupBuildDeployMocks(fsMock)
	fsMock.Method.WriteFile.ExpectCalledWithExactly(
		"/root/generated_targ_build.go",
		[]byte(expectedBuildDeployOutput),
		fs.FileMode(0o644),
	).InjectReturnValues(nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != "/root/generated_targ_build.go" {
		t.Fatalf("expected generated file path, got %q", path)
	}
}

func TestGenerateFunctionWrappers_ContextRun(t *testing.T) {
	fsMock := MockFileSystem(t)
	done := make(chan struct{})

	var (
		path string
		err  error
	)

	go func() {
		path, err = buildtool.GenerateFunctionWrappers(fsMock.Mock, buildtool.GenerateOptions{
			Dir:        "/root",
			BuildTag:   "targ",
			OnlyTagged: true,
		})

		close(done)
	}()

	setupContextRunMocks(fsMock)
	fsMock.Method.WriteFile.ExpectCalledWithExactly(
		"/root/generated_targ_build.go",
		[]byte(expectedContextRunOutput),
		fs.FileMode(0o644),
	).InjectReturnValues(nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != "/root/generated_targ_build.go" {
		t.Fatalf("expected generated file path, got %q", path)
	}
}

func TestGenerateFunctionWrappers_DefaultDirAndTag(t *testing.T) {
	fsMock := MockFileSystem(t)
	done := make(chan struct{})

	var err error

	go func() {
		// Empty Dir and empty BuildTag with OnlyTagged=true should use defaults
		_, err = buildtool.GenerateFunctionWrappers(fsMock.Mock, buildtool.GenerateOptions{
			Dir:        "",
			BuildTag:   "",
			OnlyTagged: true,
		})

		close(done)
	}()

	// Dir defaults to "." and BuildTag defaults to "targ"
	fsMock.Method.ReadDir.ExpectCalledWithExactly(".").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "readme.md", dir: false},
	}, nil)

	<-done

	if err == nil {
		t.Fatal("expected error for no Go files")
	}

	if !strings.Contains(err.Error(), "no Go files") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateFunctionWrappers_ErrorsOnNameCollision(t *testing.T) {
	fsMock := MockFileSystem(t)
	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.GenerateFunctionWrappers(fsMock.Mock, buildtool.GenerateOptions{
			Dir:        "/root",
			BuildTag:   "targ",
			OnlyTagged: true,
		})

		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

type BuildCommand struct{}

func Build() {}
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected name collision error")
	}
}

func TestGenerateFunctionWrappers_InvalidFunctionSignature(t *testing.T) {
	fsMock := MockFileSystem(t)
	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.GenerateFunctionWrappers(fsMock.Mock, buildtool.GenerateOptions{
			Dir:        "/root",
			BuildTag:   "targ",
			OnlyTagged: true,
		})

		close(done)
	}()

	// Function with too many parameters should fail validation
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

// InvalidFunc has too many parameters.
func InvalidFunc(a, b int) {}
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected error for invalid function signature")
	}

	if !strings.Contains(err.Error(), "InvalidFunc") {
		t.Fatalf("expected error to mention function name, got: %v", err)
	}
}

func TestGenerateFunctionWrappers_MultiplePackageNames(t *testing.T) {
	fsMock := MockFileSystem(t)

	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.GenerateFunctionWrappers(fsMock.Mock, buildtool.GenerateOptions{
			Dir:        "/root",
			BuildTag:   "targ",
			OnlyTagged: true,
		})

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
}

func TestGenerateFunctionWrappers_NoGoFiles(t *testing.T) {
	fsMock := MockFileSystem(t)
	done := make(chan struct{})

	var err error

	go func() {
		_, err = buildtool.GenerateFunctionWrappers(fsMock.Mock, buildtool.GenerateOptions{
			Dir:        "/root",
			BuildTag:   "targ",
			OnlyTagged: true,
		})

		close(done)
	}()

	// Return empty directory - no .go files
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "readme.md", dir: false},
	}, nil)

	<-done

	if err == nil {
		t.Fatal("expected error for no Go files")
	}

	if !strings.Contains(err.Error(), "no Go files") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateFunctionWrappers_SkipsSubcommandFunctions(t *testing.T) {
	fsMock := MockFileSystem(t)
	done := make(chan struct{})

	var (
		path string
		err  error
	)

	go func() {
		path, err = buildtool.GenerateFunctionWrappers(fsMock.Mock, buildtool.GenerateOptions{
			Dir:        "/root",
			BuildTag:   "targ",
			OnlyTagged: true,
		})

		close(done)
	}()

	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

type Root struct {
	Build func() `+"`targ:\"subcommand\"`"+`
}

func Build() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != "" {
		t.Fatalf("expected no generated file, got %q", path)
	}
}

// unexported constants.
const (
	expectedBuildDeployOutput = `//go:build targ

// Code generated by targ. DO NOT EDIT.

package build

type BuildCommand struct{}

func (c *BuildCommand) Run() {
	Build()
}

func (c *BuildCommand) Name() string {
	return "Build"
}

func (c *BuildCommand) Description() string {
	return "Build does work."
}

type DeployCommand struct{}

func (c *DeployCommand) Run() error {
	return Deploy()
}

func (c *DeployCommand) Name() string {
	return "Deploy"
}

func (c *DeployCommand) Description() string {
	return "Deploy does more."
}
`
	expectedContextRunOutput = `//go:build targ

// Code generated by targ. DO NOT EDIT.

package build

import "context"

type RunCommand struct{}

func (c *RunCommand) Run(ctx context.Context) {
	Run(ctx)
}

func (c *RunCommand) Name() string {
	return "Run"
}

func (c *RunCommand) Description() string {
	return "Run it."
}
`
)

func setupBuildDeployMocks(fsMock *FileSystemMockHandle) {
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
		fakeDirEntry{name: "other.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

// Build does work.
func Build() {}

// Deploy does more.
func Deploy() error { return nil }
`), nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/other.go").
		InjectReturnValues([]byte(`package other

func Ignored() {}
`), nil)
}

func setupContextRunMocks(fsMock *FileSystemMockHandle) {
	fsMock.Method.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.Method.ReadFile.ExpectCalledWithExactly("/root/cmd.go").
		InjectReturnValues([]byte(`//go:build targ

package build

import "context"

// Run it.
func Run(ctx context.Context) {}
`), nil)
}
