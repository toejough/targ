package core_test

// LEGACY_TESTS: This file contains tests being evaluated for redundancy.
// Property-based replacements are in *_properties_test.go files.
// Do not add new tests here. See docs/test-migration.md for details.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/toejough/targ/internal/core"
)

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

func TestRunWithEnv_CaretResetsToRoot(t *testing.T) { //nolint:paralleltest // uses global counters
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

func TestRunWithEnv_ContextFunction(t *testing.T) { //nolint:paralleltest // uses global state
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

func TestRunWithEnv_DisableHelp(t *testing.T) {
	t.Parallel()

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

func TestRunWithEnv_FunctionReturnsError(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	called := false
	helloTarget := core.Targ(func() { called = true }).Name("hello")
	root := core.Group("root", helloTarget)

	_, err := core.Execute([]string{"cmd", "hello"}, root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected function subcommand to be called")
	}
}

func TestRunWithEnv_FunctionWithHelpFlag(t *testing.T) { //nolint:paralleltest // uses global state
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

func TestRunWithEnv_GlobDoubleStarRecursive(t *testing.T) { //nolint:paralleltest // uses global counters
	multiSubOneCalls = 0
	multiSubTwoCalls = 0

	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "**"},
		core.RunOptions{AllowDefault: false},
		multiSubRootGroup(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ** should match the group and its subcommands
	if multiSubOneCalls != 1 {
		t.Fatalf("expected One to run once, got %d", multiSubOneCalls)
	}

	if multiSubTwoCalls != 1 {
		t.Fatalf("expected Two to run once, got %d", multiSubTwoCalls)
	}
}

func TestRunWithEnv_GlobNoMatches(t *testing.T) { //nolint:paralleltest // uses global counters
	testACalls = 0

	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "nonexistent-*"},
		core.RunOptions{AllowDefault: false},
		testATarget(),
	)
	if err == nil {
		t.Fatal("expected error when no targets match glob pattern")
	}

	var exitErr core.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}

	if exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.Code)
	}
}

// --- Parallel execution with globs ---

func TestRunWithEnv_GlobParallelExecution(t *testing.T) { //nolint:paralleltest // uses global counters
	testACalls = 0
	testBCalls = 0
	buildCalls = 0

	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "--parallel", "test-*"},
		core.RunOptions{AllowDefault: false},
		testATarget(),
		testBTarget(),
		buildTarget(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if testACalls != 1 {
		t.Fatalf("expected test-a to run once, got %d", testACalls)
	}

	if testBCalls != 1 {
		t.Fatalf("expected test-b to run once, got %d", testBCalls)
	}

	if buildCalls != 0 {
		t.Fatalf("expected build not to run, got %d", buildCalls)
	}
}

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

func TestRunWithEnv_GlobPrefixMatches(t *testing.T) { //nolint:paralleltest // uses global counters
	testACalls = 0
	testBCalls = 0
	buildCalls = 0

	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "test-*"},
		core.RunOptions{AllowDefault: false},
		testATarget(),
		testBTarget(),
		buildTarget(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if testACalls != 1 {
		t.Fatalf("expected test-a to run once, got %d", testACalls)
	}

	if testBCalls != 1 {
		t.Fatalf("expected test-b to run once, got %d", testBCalls)
	}

	if buildCalls != 0 {
		t.Fatalf("expected build not to run, got %d", buildCalls)
	}
}

func TestRunWithEnv_GlobStarMatchesAll(t *testing.T) { //nolint:paralleltest // uses global counters
	testACalls = 0
	testBCalls = 0
	buildCalls = 0

	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "*"},
		core.RunOptions{AllowDefault: false},
		testATarget(),
		testBTarget(),
		buildTarget(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if testACalls != 1 {
		t.Fatalf("expected test-a to run once, got %d", testACalls)
	}

	if testBCalls != 1 {
		t.Fatalf("expected test-b to run once, got %d", testBCalls)
	}

	if buildCalls != 1 {
		t.Fatalf("expected build to run once, got %d", buildCalls)
	}
}

func TestRunWithEnv_GlobSubcommands(t *testing.T) { //nolint:paralleltest // uses global counters
	multiSubOneCalls = 0
	multiSubTwoCalls = 0

	_, err := core.Execute([]string{"cmd", "*"}, multiSubRootGroup())
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

func TestRunWithEnv_GlobSuffixMatches(t *testing.T) { //nolint:paralleltest // uses global counters
	testACalls = 0
	buildCalls = 0
	deployACalls = 0

	_, err := core.ExecuteWithOptions(
		[]string{"cmd", "*-a"},
		core.RunOptions{AllowDefault: false},
		testATarget(),
		buildTarget(),
		deployATarget(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if testACalls != 1 {
		t.Fatalf("expected test-a to run once, got %d", testACalls)
	}

	if deployACalls != 1 {
		t.Fatalf("expected deploy-a to run once, got %d", deployACalls)
	}

	if buildCalls != 0 {
		t.Fatalf("expected build not to run, got %d", buildCalls)
	}
}

func TestRunWithEnv_GlobalHelpMultipleRoots(t *testing.T) {
	t.Parallel()

	// Test --help with multiple roots (no default) - shows usage
	result, err := core.ExecuteWithOptions(
		[]string{"cmd", "--help"},
		core.RunOptions{AllowDefault: false},
		DefaultFunc,
		OtherFunc,
	)
	if err != nil {
		t.Fatalf("expected no error for --help flag, got: %v", err)
	}

	// Should show both commands in usage
	if !strings.Contains(result.Output, "default-func") || !strings.Contains(result.Output, "other-func") {
		t.Fatalf("expected help to list all commands, got: %s", result.Output)
	}
}

func TestRunWithEnv_GlobalHelpShort(t *testing.T) {
	t.Parallel()

	// Test -h (short form) at global level with default command
	result, err := core.Execute([]string{"cmd", "-h"}, DefaultFunc)
	if err != nil {
		t.Fatalf("expected no error for -h flag, got: %v", err)
	}

	if !strings.Contains(result.Output, "default-func") {
		t.Fatalf("expected help output to contain command name, got: %s", result.Output)
	}
}

func TestRunWithEnv_GlobalHelpWithArgs(t *testing.T) {
	t.Parallel()

	// Test --help with args after the flag - goes through handleGlobalHelp
	result, err := core.Execute([]string{"cmd", "--help", "default-func"}, DefaultFunc)
	if err != nil {
		t.Fatalf("expected no error for --help with args, got: %v", err)
	}

	// Should show help for the default command
	if !strings.Contains(result.Output, "default-func") {
		t.Fatalf("expected help output to contain command name, got: %s", result.Output)
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
	if !strings.Contains(result.Output, "default-func") || !strings.Contains(result.Output, "other-func") {
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

func TestRunWithEnv_GroupWithNoSubcommand(t *testing.T) {
	t.Parallel()

	helloTarget := core.Targ(func() {}).Name("hello")
	root := core.Group("root", helloTarget)

	// Call group with no subcommand - should show help (no error)
	_, err := core.Execute([]string{"cmd"}, root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestRunWithEnv_HelpShowsParallelDeps(t *testing.T) {
	t.Parallel()

	depA := core.Targ(func() {}).Name("dep-a")
	depB := core.Targ(func() {}).Name("dep-b")
	target := core.Targ(func() {}).
		Name("parallel").
		Deps(depA, depB, core.DepModeParallel)

	result, _ := core.ExecuteWithOptions(
		[]string{"cmd", "--help"},
		core.RunOptions{AllowDefault: true},
		target,
	)

	if !strings.Contains(result.Output, "parallel") {
		t.Fatalf("expected 'parallel' mode in Deps line, got: %s", result.Output)
	}
}

func TestRunWithEnv_HelpShowsRetryWithoutBackoff(t *testing.T) {
	t.Parallel()

	// Test retry without backoff
	target := core.Targ(func() {}).
		Name("retry-only").
		Retry()

	result, _ := core.ExecuteWithOptions(
		[]string{"cmd", "--help"},
		core.RunOptions{AllowDefault: true},
		target,
	)

	if !strings.Contains(result.Output, "Retry: yes") {
		t.Fatalf("expected 'Retry: yes' in help, got: %s", result.Output)
	}
	// Should NOT contain backoff when not configured
	if strings.Contains(result.Output, "backoff:") {
		t.Fatalf("expected no backoff in help when not configured, got: %s", result.Output)
	}
}

func TestRunWithEnv_MultipleRoots_SubcommandThenRoot(t *testing.T) { //nolint:paralleltest // uses global counters
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

func TestRunWithEnv_MultipleSubcommands(t *testing.T) { //nolint:paralleltest // uses global counters
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

func TestRunWithEnv_MultipleTargets_FunctionByName(t *testing.T) { //nolint:paralleltest // uses global state
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

func TestRunWithEnv_SingleFunction_DefaultCommand(t *testing.T) { //nolint:paralleltest // uses global state
	defaultFuncCalled = false

	_, err := core.Execute([]string{"cmd"}, DefaultFunc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !defaultFuncCalled {
		t.Fatal("expected function command to be called")
	}
}

func TestRunWithEnv_SingleFunction_NoDefault(t *testing.T) { //nolint:paralleltest // uses global state
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
	buildCalls             int
	defaultFuncCalled      bool
	deployACalls           int
	errFuncError           = errors.New("function error")
	helloWorldCalled       bool
	multiRootDiscoverCalls int
	multiRootFlashCalls    int
	multiSubOneCalls       int
	multiSubTwoCalls       int
	testACalls             int
	testBCalls             int
)

func buildTarget() *core.Target {
	return core.Targ(func() { buildCalls++ }).Name("build")
}

func deployATarget() *core.Target {
	return core.Targ(func() { deployACalls++ }).Name("deploy-a")
}

func discoverTarget() *core.Target {
	return core.Targ(func() { multiRootDiscoverCalls++ }).Name("discover")
}

func firmwareGroup() *core.TargetGroup {
	return core.Group("firmware", flashOnlyTarget())
}

func flashOnlyTarget() *core.Target {
	return core.Targ(func() { multiRootFlashCalls++ }).Name("flash-only")
}

func multiSubRootGroup() *core.TargetGroup {
	return core.Group("multi-sub-root", oneTarget(), twoTarget())
}

// Helper functions to create targets for tests

func oneTarget() *core.Target {
	return core.Targ(func() { multiSubOneCalls++ }).Name("one")
}

func testATarget() *core.Target {
	return core.Targ(func() { testACalls++ }).Name("test-a")
}

func testBTarget() *core.Target {
	return core.Targ(func() { testBCalls++ }).Name("test-b")
}

func twoTarget() *core.Target {
	return core.Targ(func() { multiSubTwoCalls++ }).Name("two")
}
