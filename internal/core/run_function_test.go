package core_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/toejough/targ/internal/core"
)

// Helper functions to create targets for tests

func oneTarget() *core.Target {
	return core.Targ(func() { multiSubOneCalls++ }).Name("one")
}

func twoTarget() *core.Target {
	return core.Targ(func() { multiSubTwoCalls++ }).Name("two")
}

func multiSubRootGroup() *core.Group {
	return core.NewGroup("multi-sub-root", oneTarget(), twoTarget())
}

func discoverTarget() *core.Target {
	return core.Targ(func() { multiRootDiscoverCalls++ }).Name("discover")
}

func flashOnlyTarget() *core.Target {
	return core.Targ(func() { multiRootFlashCalls++ }).Name("flash-only")
}

func firmwareGroup() *core.Group {
	return core.NewGroup("firmware", flashOnlyTarget())
}

func ContextFunc(ctx context.Context) {
	if ctx != nil {
		helloWorldCalled = true
	}
}

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

func TestRunWithEnv_CaretResetsToRoot(t *testing.T) {
	multiSubOneCalls = 0
	multiRootDiscoverCalls = 0

	_, err := core.Execute(
		[]string{"cmd", "multi-sub-root", "one", "^", "discover"},
		multiSubRootGroup(),
		discoverTarget(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if multiSubOneCalls != 1 {
		t.Fatalf("expected One to run once, got %d", multiSubOneCalls)
	}

	if multiRootDiscoverCalls != 1 {
		t.Fatalf("expected discover to run once, got %d", multiRootDiscoverCalls)
	}
}

func TestRunWithEnv_ContextFunction(t *testing.T) {
	helloWorldCalled = false

	_, err := core.Execute([]string{"cmd"}, ContextFunc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !helloWorldCalled {
		t.Fatal("expected context function command to be called")
	}
}

func TestRunWithEnv_DisableCompletion(t *testing.T) {
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

func TestRunWithEnv_DisableHelp(t *testing.T) {
	// With DisableHelp, --help should be passed through as unknown flag
	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "--help"},
		core.RunOptions{DisableHelp: true, AllowDefault: true},
		DefaultFunc,
	)
	if err == nil {
		t.Fatal("expected error when help is disabled and --help is passed")
	}

	// Should error as unknown flag
	var exitErr core.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}

	if exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.Code)
	}
}

func TestRunWithEnv_DisableTimeout(t *testing.T) {
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

func TestRunWithEnv_FunctionReturnsError(t *testing.T) {
	_, err := core.Execute([]string{"cmd"}, ErrorFunc)
	if err == nil {
		t.Fatal("expected error from function")
	}

	// Error is wrapped in ExitError
	var exitErr core.ExitError

	ok := errors.As(err, &exitErr)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}

	if exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.Code)
	}
}

func TestRunWithEnv_FunctionSubcommand(t *testing.T) {
	called := false
	helloTarget := core.Targ(func() { called = true }).Name("hello")
	root := core.NewGroup("root", helloTarget)

	_, err := core.Execute([]string{"cmd", "hello"}, root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected function subcommand to be called")
	}
}

func TestRunWithEnv_FunctionWithHelpFlag(t *testing.T) {
	defaultFuncCalled = false

	_, err := core.ExecuteWithOptions(
		[]string{"cmd"},
		core.RunOptions{AllowDefault: true, HelpOnly: true},
		DefaultFunc,
	)
	// HelpOnly should skip execution
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if defaultFuncCalled {
		t.Fatal("expected function not to be called in HelpOnly mode")
	}
}

func TestRunWithEnv_GlobalHelpMultipleRoots(t *testing.T) {
	// Test --help with multiple roots (no default) - shows usage
	output := captureStdoutRun(t, func() {
		_, err := core.ExecuteWithOptions(
			[]string{"cmd", "--help"},
			core.RunOptions{AllowDefault: false},
			DefaultFunc,
			OtherFunc,
		)
		if err != nil {
			t.Fatalf("expected no error for --help flag, got: %v", err)
		}
	})

	// Should show both commands in usage
	if !strings.Contains(output, "default-func") || !strings.Contains(output, "other-func") {
		t.Fatalf("expected help to list all commands, got: %s", output)
	}
}

func TestRunWithEnv_GlobalHelpShort(t *testing.T) {
	// Test -h (short form) at global level with default command
	output := captureStdoutRun(t, func() {
		_, err := core.Execute([]string{"cmd", "-h"}, DefaultFunc)
		if err != nil {
			t.Fatalf("expected no error for -h flag, got: %v", err)
		}
	})

	if !strings.Contains(output, "default-func") {
		t.Fatalf("expected help output to contain command name, got: %s", output)
	}
}

func TestRunWithEnv_GlobalHelpWithArgs(t *testing.T) {
	// Test --help with args after the flag - goes through handleGlobalHelp
	output := captureStdoutRun(t, func() {
		_, err := core.Execute([]string{"cmd", "--help", "default-func"}, DefaultFunc)
		if err != nil {
			t.Fatalf("expected no error for --help with args, got: %v", err)
		}
	})

	// Should show help for the default command
	if !strings.Contains(output, "default-func") {
		t.Fatalf("expected help output to contain command name, got: %s", output)
	}
}

func TestRunWithEnv_GlobalHelpWithArgsMultiRoot(t *testing.T) {
	// Test --help with args for multiple roots - goes through handleGlobalHelp else branch
	output := captureStdoutRun(t, func() {
		_, err := core.ExecuteWithOptions(
			[]string{"cmd", "--help", "something"},
			core.RunOptions{AllowDefault: false},
			DefaultFunc,
			OtherFunc,
		)
		if err != nil {
			t.Fatalf("expected no error for --help with args, got: %v", err)
		}
	})

	// Should show usage listing all commands
	if !strings.Contains(output, "default-func") || !strings.Contains(output, "other-func") {
		t.Fatalf("expected help to list all commands, got: %s", output)
	}
}

func TestRunWithEnv_MultipleRoots_SubcommandThenRoot(t *testing.T) {
	multiRootFlashCalls = 0
	multiRootDiscoverCalls = 0

	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "firmware", "flash-only", "discover"},
		core.RunOptions{AllowDefault: false},
		firmwareGroup(),
		discoverTarget(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if multiRootFlashCalls != 1 {
		t.Fatalf("expected flash-only to run once, got %d", multiRootFlashCalls)
	}

	if multiRootDiscoverCalls != 1 {
		t.Fatalf("expected discover to run once, got %d", multiRootDiscoverCalls)
	}
}

func TestRunWithEnv_MultipleSubcommands(t *testing.T) {
	multiSubOneCalls = 0
	multiSubTwoCalls = 0

	_, err := core.Execute([]string{"cmd", "one", "two"}, multiSubRootGroup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if multiSubOneCalls != 1 {
		t.Fatalf("expected One to run once, got %d", multiSubOneCalls)
	}

	if multiSubTwoCalls != 1 {
		t.Fatalf("expected Two to run once, got %d", multiSubTwoCalls)
	}
}

func TestRunWithEnv_MultipleTargets_FunctionByName(t *testing.T) {
	helloWorldCalled = false
	otherTarget := core.Targ(func() {}).Name("other")

	_, err := core.Execute([]string{"cmd", "hello-world"}, HelloWorld, otherTarget)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !helloWorldCalled {
		t.Fatal("expected function command to be called")
	}
}

func TestRunWithEnv_SingleFunction_DefaultCommand(t *testing.T) {
	defaultFuncCalled = false

	_, err := core.Execute([]string{"cmd"}, DefaultFunc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !defaultFuncCalled {
		t.Fatal("expected function command to be called")
	}
}

func TestRunWithEnv_SingleFunction_NoDefault(t *testing.T) {
	defaultFuncCalled = false

	_, err := core.ExecuteWithOptions(
		[]string{"cmd"},
		core.RunOptions{AllowDefault: false},
		DefaultFunc,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if defaultFuncCalled {
		t.Fatal("expected function command not to be called without default")
	}
}

// unexported variables.
var (
	defaultFuncCalled      bool
	errFuncError           = errors.New("function error")
	helloWorldCalled       bool
	multiRootDiscoverCalls int
	multiRootFlashCalls    int
	multiSubOneCalls       int
	multiSubTwoCalls       int
)

func captureStdoutRun(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("unexpected pipe error: %v", err)
	}

	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer

	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("unexpected stdout copy error: %v", err)
	}

	_ = r.Close()

	return buf.String()
}
