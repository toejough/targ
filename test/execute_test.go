package targ_test

// LEGACY_TESTS: This file contains tests being evaluated for redundancy.
// Property-based replacements are in *_properties_test.go files.
// Do not add new tests here. See docs/test-migration.md for details.

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/toejough/targ"
)

type ExecuteTestArgs struct {
	Name string `targ:"flag"`
}

// Args struct types for Target functions (these stay - they have no Run() method).

type InterleavedFlagsArgs struct {
	Include []targ.Interleaved[string] `targ:"flag"`
	Exclude []targ.Interleaved[string] `targ:"flag"`
}

type InterleavedIntArgs struct {
	Values []targ.Interleaved[int] `targ:"flag"`
}

type MapStringIntArgs struct {
	Ports map[string]int `targ:"flag"`
}

type MapStringStringArgs struct {
	Labels map[string]string `targ:"flag"`
}

type RepeatedFlagsArgs struct {
	Tags []string `targ:"flag"`
}

type RepeatedIntFlagsArgs struct {
	Counts []int `targ:"flag"`
}

func TestExecuteWithOptions_AllowDefault(t *testing.T) {
	t.Parallel()

	called := false
	target := targ.Targ(func() { called = true })

	_, err := targ.ExecuteWithOptions([]string{"app"}, targ.RunOptions{AllowDefault: true}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected default command to be called")
	}
}

func TestExecuteWithOptions_NoDefaultShowsUsage(t *testing.T) {
	t.Parallel()

	called := false
	target := targ.Targ(func() { called = true })

	_, err := targ.ExecuteWithOptions([]string{"app"}, targ.RunOptions{AllowDefault: false}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if called {
		t.Fatal("expected command NOT to be called without AllowDefault")
	}
}

func TestExecute_CommandError(t *testing.T) {
	t.Parallel()

	target := targ.Targ(func() error {
		return errors.New("command failed")
	})

	result, err := targ.Execute([]string{"app"}, target)
	if err == nil {
		t.Fatal("expected error from command")
	}

	var exitErr targ.ExitError

	ok := errors.As(err, &exitErr)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}

	if exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.Code)
	}

	if !strings.Contains(result.Output, "command failed") {
		t.Fatalf("expected error message in output, got: %q", result.Output)
	}
}

func TestExecute_Success(t *testing.T) {
	t.Parallel()

	var gotName string

	called := false

	target := targ.Targ(func(args ExecuteTestArgs) {
		called = true
		gotName = args.Name
	})

	_, err := targ.Execute([]string{"app", "--name", testNameAlice}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected command to be called")
	}

	if gotName != testNameAlice {
		t.Fatalf("expected name=%s, got %q", testNameAlice, gotName)
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	t.Parallel()

	target := targ.Targ(func(_ ExecuteTestArgs) {}).Name("test-cmd")

	result, err := targ.ExecuteWithOptions(
		[]string{"app", "unknown"},
		targ.RunOptions{AllowDefault: false},
		target,
	)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}

	var exitErr targ.ExitError

	ok := errors.As(err, &exitErr)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}

	if exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.Code)
	}

	if !strings.Contains(result.Output, "Unknown command") {
		t.Fatalf("expected unknown command message, got: %q", result.Output)
	}
}

func TestGroup_AsNamedRoot(t *testing.T) {
	t.Parallel()

	called := false
	sub := targ.Targ(func() { called = true }).Name("sub")
	group := targ.Group("grp", sub)
	other := targ.Targ(func() {}).Name("other")

	// Group as one of multiple roots - need to specify group name
	_, err := targ.Execute([]string{"app", "grp", "sub"}, group, other)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected sub target to be called")
	}
}

func TestGroup_MultipleMembers(t *testing.T) {
	t.Parallel()

	var calledTarget string

	build := targ.Targ(func() { calledTarget = "build" }).Name("build")
	test := targ.Targ(func() { calledTarget = "test" }).Name("test")
	group := targ.Group("dev", build, test)

	// Single root group - go directly to member
	_, err := targ.Execute([]string{"app", "test"}, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calledTarget != "test" {
		t.Fatalf("expected 'test' to be called, got %q", calledTarget)
	}
}

func TestGroup_NestedRouting(t *testing.T) {
	t.Parallel()

	called := false
	inner := targ.Targ(func() { called = true }).Name("inner")
	innerGroup := targ.Group("inner-grp", inner)
	outerGroup := targ.Group("outer", innerGroup)

	// Single root (outer) - route through inner-grp to inner
	_, err := targ.Execute([]string{"app", "inner-grp", "inner"}, outerGroup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected inner target to be called")
	}
}

func TestGroup_RoutesToMembers(t *testing.T) {
	t.Parallel()

	called := false
	sub := targ.Targ(func() { called = true }).Name("sub")
	group := targ.Group("grp", sub)

	// Group as single root (default) - go directly to member name
	_, err := targ.Execute([]string{"app", "sub"}, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected sub target to be called")
	}
}

func TestInterleavedFlags_IntType(t *testing.T) {
	t.Parallel()

	var gotValues []targ.Interleaved[int]

	target := targ.Targ(func(args InterleavedIntArgs) {
		gotValues = args.Values
	})

	_, err := targ.Execute([]string{"app", "--values", "10", "--values", "20"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotValues) != 2 {
		t.Fatalf("expected 2 values, got %d", len(gotValues))
	}

	if gotValues[0].Value != 10 || gotValues[0].Position != 0 {
		t.Fatalf("expected values[0]={10,0}, got %+v", gotValues[0])
	}

	if gotValues[1].Value != 20 || gotValues[1].Position != 1 {
		t.Fatalf("expected values[1]={20,1}, got %+v", gotValues[1])
	}
}

func TestInterleavedFlags_ReconstructOrder(t *testing.T) {
	t.Parallel()

	var (
		gotIncludes []targ.Interleaved[string]
		gotExcludes []targ.Interleaved[string]
	)

	target := targ.Targ(func(args InterleavedFlagsArgs) {
		gotIncludes = args.Include
		gotExcludes = args.Exclude
	})

	_, err := targ.Execute(
		[]string{"app", "--exclude", "x", "--include", "a", "--include", "b", "--exclude", "y"},
		target,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Collect all items with their metadata
	type item struct {
		position int
	}

	all := make([]item, 0, len(gotIncludes)+len(gotExcludes))
	for _, inc := range gotIncludes {
		all = append(all, item{inc.Position})
	}

	for _, exc := range gotExcludes {
		all = append(all, item{exc.Position})
	}

	slices.SortFunc(all, func(a, b item) int { return a.position - b.position })

	expected := []item{
		{0},
		{1},
		{2},
		{3},
	}

	if !slices.Equal(all, expected) {
		t.Fatalf("expected %+v, got %+v", expected, all)
	}
}

func TestInterleavedFlags_TracksPosition(t *testing.T) {
	t.Parallel()

	var (
		gotIncludes []targ.Interleaved[string]
		gotExcludes []targ.Interleaved[string]
	)

	target := targ.Targ(func(args InterleavedFlagsArgs) {
		gotIncludes = args.Include
		gotExcludes = args.Exclude
	})

	_, err := targ.Execute(
		[]string{"app", "--include", "a", "--exclude", "b", "--include", "c"},
		target,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotIncludes) != 2 {
		t.Fatalf("expected 2 includes, got %d: %v", len(gotIncludes), gotIncludes)
	}

	if gotIncludes[0].Value != "a" || gotIncludes[0].Position != 0 {
		t.Fatalf("expected include[0]={a,0}, got %+v", gotIncludes[0])
	}

	if gotIncludes[1].Value != "c" || gotIncludes[1].Position != 2 {
		t.Fatalf("expected include[1]={c,2}, got %+v", gotIncludes[1])
	}

	if len(gotExcludes) != 1 {
		t.Fatalf("expected 1 exclude, got %d: %v", len(gotExcludes), gotExcludes)
	}

	if gotExcludes[0].Value != "b" || gotExcludes[0].Position != 1 {
		t.Fatalf("expected exclude[0]={b,1}, got %+v", gotExcludes[0])
	}
}

func TestMapFlags_InvalidFormat(t *testing.T) {
	t.Parallel()

	target := targ.Targ(func(_ MapStringStringArgs) {})

	_, err := targ.Execute([]string{"app", "--labels", "invalid"}, target)
	if err == nil {
		t.Fatal("expected error for invalid map format")
	}
}

func TestMapFlags_OverwriteKey(t *testing.T) {
	t.Parallel()

	var gotLabels map[string]string

	target := targ.Targ(func(args MapStringStringArgs) {
		gotLabels = args.Labels
	})

	_, err := targ.Execute([]string{"app", "--labels", "env=dev", "--labels", "env=prod"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotLabels["env"] != "prod" {
		t.Fatalf("expected env=prod (overwritten), got %q", gotLabels["env"])
	}
}

func TestMapFlags_StringInt(t *testing.T) {
	t.Parallel()

	var gotPorts map[string]int

	target := targ.Targ(func(args MapStringIntArgs) {
		gotPorts = args.Ports
	})

	_, err := targ.Execute([]string{"app", "--ports", "http=80", "--ports", "https=443"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotPorts) != 2 {
		t.Fatalf("expected 2 ports, got %d: %v", len(gotPorts), gotPorts)
	}

	if gotPorts["http"] != 80 {
		t.Fatalf("expected http=80, got %d", gotPorts["http"])
	}

	if gotPorts["https"] != 443 {
		t.Fatalf("expected https=443, got %d", gotPorts["https"])
	}
}

func TestMapFlags_StringString(t *testing.T) {
	t.Parallel()

	var gotLabels map[string]string

	target := targ.Targ(func(args MapStringStringArgs) {
		gotLabels = args.Labels
	})

	_, err := targ.Execute([]string{"app", "--labels", "env=prod", "--labels", "app=web"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotLabels) != 2 {
		t.Fatalf("expected 2 labels, got %d: %v", len(gotLabels), gotLabels)
	}

	if gotLabels["env"] != "prod" {
		t.Fatalf("expected env=prod, got %q", gotLabels["env"])
	}

	if gotLabels["app"] != "web" {
		t.Fatalf("expected app=web, got %q", gotLabels["app"])
	}
}

func TestMapFlags_ValueWithEquals(t *testing.T) {
	t.Parallel()

	var gotLabels map[string]string

	target := targ.Targ(func(args MapStringStringArgs) {
		gotLabels = args.Labels
	})

	_, err := targ.Execute([]string{"app", "--labels", "equation=a=b"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotLabels["equation"] != "a=b" {
		t.Fatalf("expected equation=a=b, got %q", gotLabels["equation"])
	}
}

func TestMultipleRoots_RoutesCorrectly(t *testing.T) {
	t.Parallel()

	var calledTarget string

	build := targ.Targ(func() { calledTarget = "build" }).Name("build")
	test := targ.Targ(func() { calledTarget = "test" }).Name("test")

	_, err := targ.Execute([]string{"app", "test"}, build, test)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calledTarget != "test" {
		t.Fatalf("expected 'test' to be called, got %q", calledTarget)
	}
}

func TestRepeatedFlags_Accumulates(t *testing.T) {
	t.Parallel()

	var gotTags []string

	target := targ.Targ(func(args RepeatedFlagsArgs) {
		gotTags = args.Tags
	})

	_, err := targ.Execute([]string{"app", "--tags", "a", "--tags", "b", "--tags", "c"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotTags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(gotTags), gotTags)
	}

	if gotTags[0] != "a" || gotTags[1] != "b" || gotTags[2] != "c" {
		t.Fatalf("unexpected tags order: %v", gotTags)
	}
}

func TestRepeatedFlags_IntSlice(t *testing.T) {
	t.Parallel()

	var gotCounts []int

	target := targ.Targ(func(args RepeatedIntFlagsArgs) {
		gotCounts = args.Counts
	})

	_, err := targ.Execute(
		[]string{"app", "--counts", "1", "--counts", "2", "--counts", "3"},
		target,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotCounts) != 3 {
		t.Fatalf("expected 3 counts, got %d: %v", len(gotCounts), gotCounts)
	}

	if gotCounts[0] != 1 || gotCounts[1] != 2 || gotCounts[2] != 3 {
		t.Fatalf("unexpected counts: %v", gotCounts)
	}
}

func TestStringTarget_NoVars_ExecutesDirectly(t *testing.T) {
	t.Parallel()

	// String target without variables should just execute
	target := targ.Targ("true").Name("simple-cmd")

	result, err := targ.Execute([]string{"app"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

func TestStringTarget_WithVars_EqualsValueSyntax(t *testing.T) {
	t.Parallel()

	// Test --flag=value syntax
	target := targ.Targ("echo $name $port").Name("test-cmd")

	result, err := targ.Execute([]string{"app", "--name=hello", "--port=8080"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

func TestStringTarget_WithVars_InfersFlags(t *testing.T) {
	t.Parallel()

	// String target with $var should create flags from variables
	target := targ.Targ("echo $name $port").Name("test-cmd")

	// Execute with --name and --port flags (inferred from $vars)
	// Note: Single root = default, so no command name needed
	result, err := targ.Execute([]string{"app", "--name", "hello", "--port", "8080"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

func TestStringTarget_WithVars_MissingRequiredFlag(t *testing.T) {
	t.Parallel()

	target := targ.Targ("echo $name $port").Name("test-cmd")

	// Missing --port should fail
	_, err := targ.Execute([]string{"app", "--name", "hello"}, target)
	if err == nil {
		t.Fatal("expected error for missing required flag")
	}
}

func TestStringTarget_WithVars_MixedSyntax(t *testing.T) {
	t.Parallel()

	// Test mixing --flag=value and --flag value syntax
	target := targ.Targ("echo $name $port").Name("test-cmd")

	// Mix equals syntax with space syntax
	result, err := targ.Execute([]string{"app", "--name=hello", "--port", "8080"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

func TestStringTarget_WithVars_MultiRoot(t *testing.T) {
	t.Parallel()

	// With multiple roots, need to specify command name
	target := targ.Targ("echo $name").Name("echo-cmd")
	other := targ.Targ(func() {}).Name("other")

	result, err := targ.Execute([]string{"app", "echo-cmd", "--name", "hello"}, target, other)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

func TestStringTarget_WithVars_ShortFlags(t *testing.T) {
	t.Parallel()

	// First letters should become short flags
	target := targ.Targ("echo $name $port").Name("test-cmd")

	// Use short flags -n and -p
	result, err := targ.Execute([]string{"app", "-n", "hello", "-p", "8080"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

// --- Target/Group Tests ---

func TestTarget_BasicExecution(t *testing.T) {
	t.Parallel()

	called := false
	target := targ.Targ(func() { called = true })

	_, err := targ.Execute([]string{"app"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected target function to be called")
	}
}

func TestTarget_CustomName_RoutesWithMultipleRoots(t *testing.T) {
	t.Parallel()

	var calledTarget string

	first := targ.Targ(func() { calledTarget = "first" }).Name("first")
	custom := targ.Targ(func() { calledTarget = "custom" }).Name("my-target")

	_, err := targ.Execute([]string{"app", "my-target"}, first, custom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calledTarget != "custom" {
		t.Fatalf("expected 'custom' to be called, got %q", calledTarget)
	}
}

func TestTarget_WithArgs(t *testing.T) {
	t.Parallel()

	type Args struct {
		Name string `targ:"flag"`
	}

	var gotName string

	target := targ.Targ(func(args Args) {
		gotName = args.Name
	})

	result, err := targ.Execute([]string{"app", "--name", "test"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}

	if gotName != "test" {
		t.Fatalf("expected name='test', got %q", gotName)
	}
}

func TestTarget_WithEmbeddedArgs(t *testing.T) {
	t.Parallel()

	type CommonArgs struct {
		Verbose bool `targ:"flag,short=v"`
	}

	type DeployArgs struct {
		CommonArgs

		Env string `targ:"flag"`
	}

	var (
		gotVerbose bool
		gotEnv     string
	)

	target := targ.Targ(func(args DeployArgs) {
		gotVerbose = args.Verbose
		gotEnv = args.Env
	})

	result, err := targ.Execute([]string{"app", "--verbose", "--env", "prod"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}

	if !gotVerbose {
		t.Fatal("expected verbose=true")
	}

	if gotEnv != "prod" {
		t.Fatalf("expected env='prod', got %q", gotEnv)
	}
}

func TestTimeout_EqualsStyle(t *testing.T) {
	t.Parallel()

	called := false

	target := targ.Targ(func(ctx context.Context) error {
		called = true

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil
		}
	}).Name("timeout-cmd")

	_, err := targ.Execute([]string{"app", "--timeout=1s"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_Exceeded(t *testing.T) {
	t.Parallel()

	target := targ.Targ(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	}).Name("slow-cmd")

	_, err := targ.Execute([]string{"app", "--timeout", "10ms"}, target)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestTimeout_GlobalAndPerCommand(t *testing.T) {
	t.Parallel()

	called := false

	target := targ.Targ(func(ctx context.Context) error {
		called = true

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil
		}
	}).Name("timeout-cmd")

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "--timeout", "5s", "timeout-cmd", "--timeout", "1s"},
		targ.RunOptions{AllowDefault: false},
		target,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_InvalidDuration(t *testing.T) {
	t.Parallel()

	target := targ.Targ(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil
		}
	}).Name("timeout-cmd")

	_, err := targ.Execute([]string{"app", "--timeout", "invalid"}, target)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestTimeout_MultiCommandDifferentTimeouts(t *testing.T) {
	t.Parallel()

	fastCalled := false
	slowCalled := false

	fast := targ.Targ(func(_ context.Context) error {
		fastCalled = true
		return nil
	}).Name("fast-cmd")

	slow := targ.Targ(func(ctx context.Context) error {
		slowCalled = true

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	}).Name("slow-cmd")

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "fast-cmd", "--timeout", "10ms", "slow-cmd", "--timeout", "2s"},
		targ.RunOptions{AllowDefault: false},
		fast, slow,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fastCalled {
		t.Fatal("expected fast command to be called")
	}

	if !slowCalled {
		t.Fatal("expected slow command to be called")
	}
}

func TestTimeout_NotExceeded(t *testing.T) {
	t.Parallel()

	called := false

	target := targ.Targ(func(ctx context.Context) error {
		called = true

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil
		}
	}).Name("timeout-cmd")

	_, err := targ.Execute([]string{"app", "--timeout", "1s"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_PerCommand(t *testing.T) {
	t.Parallel()

	called := false

	target := targ.Targ(func(ctx context.Context) error {
		called = true

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil
		}
	}).Name("timeout-cmd")

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "timeout-cmd", "--timeout", "1s"},
		targ.RunOptions{AllowDefault: false},
		target,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_PerCommandExceeded(t *testing.T) {
	t.Parallel()

	target := targ.Targ(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	}).Name("slow-cmd")

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "slow-cmd", "--timeout", "10ms"},
		targ.RunOptions{AllowDefault: false},
		target,
	)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// unexported constants.
const (
	testNameAlice = "Alice"
)
