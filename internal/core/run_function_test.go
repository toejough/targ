package core_test

// LEGACY_TESTS: This file contains tests being evaluated for redundancy.
// Property-based replacements are in *_properties_test.go files.
// Do not add new tests here. See docs/test-migration.md for details.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/toejough/targ/internal/core"
)

func DefaultFunc() {
	defaultFuncCalled = true
}

// --- Error and special case tests ---

func ErrorFunc() error {
	return errFuncError
}

func HelloWorld() {
	helloWorldCalled = true
}

func OtherFunc() {
	// Another function for multi-root tests
}

func TestRunWithEnv_DisableCompletion(t *testing.T) {
	t.Parallel()

	// With DisableCompletion, --completion should be passed through as unknown flag
	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "--completion", "bash"},
		core.RunOptions{DisableCompletion: true, AllowDefault: true},
		DefaultFunc,
	)
	if err == nil {
		t.Fatal("expected error when completion is disabled and --completion is passed")
	}
}

func TestRunWithEnv_DisableTimeout(t *testing.T) {
	t.Parallel()

	// With DisableTimeout, --timeout should be passed through as unknown flag
	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "--timeout", "5m"},
		core.RunOptions{DisableTimeout: true, AllowDefault: true},
		DefaultFunc,
	)
	if err == nil {
		t.Fatal("expected error when timeout is disabled and --timeout is passed")
	}
}

// --- Parallel execution with globs ---

func TestRunWithEnv_GlobParallelNoMatches(t *testing.T) {
	t.Parallel()

	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "--parallel", "nonexistent-*"},
		core.RunOptions{AllowDefault: false},
		testATarget(),
	)
	if err == nil {
		t.Fatal("expected error when no targets match glob pattern in parallel mode")
	}

	var exitErr core.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
}

func TestRunWithEnv_GlobalHelpWithArgsMultiRoot(t *testing.T) {
	t.Parallel()

	// Test --help with args for multiple roots - goes through handleGlobalHelp else branch
	result, err := core.ExecuteWithOptions(
		[]string{"cmd", "--help", "something"},
		core.RunOptions{AllowDefault: false},
		DefaultFunc,
		OtherFunc,
	)
	if err != nil {
		t.Fatalf("expected no error for --help with args, got: %v", err)
	}

	// Should show usage listing all commands
	if !strings.Contains(result.Output, "default-func") ||
		!strings.Contains(result.Output, "other-func") {
		t.Fatalf("expected help to list all commands, got: %s", result.Output)
	}
}

func TestRunWithEnv_GroupUnknownSubcommand(t *testing.T) {
	t.Parallel()

	helloTarget := core.Targ(func() {}).Name("hello")
	root := core.Group("root", helloTarget)

	// Call group with unknown subcommand - should error
	_, err := core.Execute([]string{"cmd", "unknown"}, root)
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- Execution info in help tests ---

func TestRunWithEnv_HelpShowsExecutionInfo(t *testing.T) {
	t.Parallel()

	depTarget := core.Targ(func() {}).Name("dep")
	mainTarget := core.Targ(func() {}).
		Name("main").
		Deps(depTarget).
		Cache("*.go").
		Watch("*.go").
		Timeout(5*time.Minute).
		Times(3).
		Retry().
		Backoff(time.Second, 2.0)

	// Use single target with AllowDefault to get command-specific help
	result, _ := core.ExecuteWithOptions(
		[]string{"cmd", "--help"},
		core.RunOptions{AllowDefault: true},
		mainTarget,
	)

	// Check for execution info sections
	if !strings.Contains(result.Output, "Execution:") {
		t.Fatalf("expected Execution section in help, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "Deps:") {
		t.Fatalf("expected Deps line in help, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "Cache:") {
		t.Fatalf("expected Cache line in help, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "Watch:") {
		t.Fatalf("expected Watch line in help, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "Timeout:") {
		t.Fatalf("expected Timeout line in help, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "Times:") {
		t.Fatalf("expected Times line in help, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "Retry:") {
		t.Fatalf("expected Retry line in help, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "backoff:") {
		t.Fatalf("expected backoff info in help, got: %s", result.Output)
	}
}

// unexported variables.
var (
	defaultFuncCalled bool
	errFuncError      = errors.New("function error")
	helloWorldCalled  bool
	testACalls        int
)

// Helper functions to create targets for tests

func testATarget() *core.Target {
	return core.Targ(func() { testACalls++ }).Name("test-a")
}
