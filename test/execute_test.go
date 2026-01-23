package targ_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/toejough/targ"
)

type ExecuteDefaultCmd struct {
	Called bool
}

func (c *ExecuteDefaultCmd) Run() {
	c.Called = true
}

type ExecuteErrorCmd struct{}

func (c *ExecuteErrorCmd) Run() error {
	return errors.New("command failed")
}

type ExecuteTestCmd struct {
	Name   string `targ:"flag"`
	Called bool
}

func (c *ExecuteTestCmd) Run() {
	c.Called = true
}

type FastCmd struct {
	Called bool
}

func (c *FastCmd) Run(_ context.Context) error {
	c.Called = true
	return nil
}

type InterleavedFlagsCmd struct {
	Include []targ.Interleaved[string] `targ:"flag"`
	Exclude []targ.Interleaved[string] `targ:"flag"`
}

func (c *InterleavedFlagsCmd) Run() {}

type InterleavedIntCmd struct {
	Values []targ.Interleaved[int] `targ:"flag"`
}

func (c *InterleavedIntCmd) Run() {}

type MapStringIntCmd struct {
	Ports map[string]int `targ:"flag"`
}

func (c *MapStringIntCmd) Run() {}

type MapStringStringCmd struct {
	Labels map[string]string `targ:"flag"`
}

func (c *MapStringStringCmd) Run() {}

type RepeatedFlagsCmd struct {
	Tags []string `targ:"flag"`
}

func (c *RepeatedFlagsCmd) Run() {}

type RepeatedIntFlagsCmd struct {
	Counts []int `targ:"flag"`
}

func (c *RepeatedIntFlagsCmd) Run() {}

type SlowCmd struct {
	Called bool
}

func (c *SlowCmd) Run(ctx context.Context) error {
	c.Called = true

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(1 * time.Second):
		return nil
	}
}

type TimeoutCmd struct {
	Called bool
}

func (c *TimeoutCmd) Run(ctx context.Context) error {
	c.Called = true

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}

func TestExecuteWithOptions_AllowDefault(t *testing.T) {
	t.Parallel()

	cmd := &ExecuteDefaultCmd{}

	_, err := targ.ExecuteWithOptions([]string{"app"}, targ.RunOptions{AllowDefault: true}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cmd.Called {
		t.Fatal("expected default command to be called")
	}
}

func TestExecuteWithOptions_NoDefaultShowsUsage(t *testing.T) {
	t.Parallel()

	cmd := &ExecuteDefaultCmd{}

	_, err := targ.ExecuteWithOptions([]string{"app"}, targ.RunOptions{AllowDefault: false}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Called {
		t.Fatal("expected command NOT to be called without AllowDefault")
	}
}

func TestExecute_CommandError(t *testing.T) {
	t.Parallel()

	cmd := &ExecuteErrorCmd{}

	result, err := targ.Execute([]string{"app"}, cmd)
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

	cmd := &ExecuteTestCmd{}

	_, err := targ.Execute([]string{"app", "--name", testNameAlice}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cmd.Called {
		t.Fatal("expected command to be called")
	}

	if cmd.Name != testNameAlice {
		t.Fatalf("expected name=%s, got %q", testNameAlice, cmd.Name)
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	t.Parallel()

	cmd := &ExecuteTestCmd{}

	result, err := targ.ExecuteWithOptions(
		[]string{"app", "unknown"},
		targ.RunOptions{AllowDefault: false},
		cmd,
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
	called := false
	sub := targ.Targ(func() { called = true }).Name("sub")
	group := targ.NewGroup("grp", sub)
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
	var calledTarget string

	build := targ.Targ(func() { calledTarget = "build" }).Name("build")
	test := targ.Targ(func() { calledTarget = "test" }).Name("test")
	group := targ.NewGroup("dev", build, test)

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
	called := false
	inner := targ.Targ(func() { called = true }).Name("inner")
	innerGroup := targ.NewGroup("inner-grp", inner)
	outerGroup := targ.NewGroup("outer", innerGroup)

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
	called := false
	sub := targ.Targ(func() { called = true }).Name("sub")
	group := targ.NewGroup("grp", sub)

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
	cmd := &InterleavedIntCmd{}

	_, err := targ.Execute([]string{"app", "--values", "10", "--values", "20"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmd.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(cmd.Values))
	}

	if cmd.Values[0].Value != 10 || cmd.Values[0].Position != 0 {
		t.Fatalf("expected values[0]={10,0}, got %+v", cmd.Values[0])
	}

	if cmd.Values[1].Value != 20 || cmd.Values[1].Position != 1 {
		t.Fatalf("expected values[1]={20,1}, got %+v", cmd.Values[1])
	}
}

func TestInterleavedFlags_ReconstructOrder(t *testing.T) {
	cmd := &InterleavedFlagsCmd{}

	_, err := targ.Execute(
		[]string{"app", "--exclude", "x", "--include", "a", "--include", "b", "--exclude", "y"},
		cmd,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Collect all items with their metadata
	type item struct {
		position int
	}

	all := make([]item, 0, len(cmd.Include)+len(cmd.Exclude))
	for _, inc := range cmd.Include {
		all = append(all, item{inc.Position})
	}

	for _, exc := range cmd.Exclude {
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
	cmd := &InterleavedFlagsCmd{}

	_, err := targ.Execute(
		[]string{"app", "--include", "a", "--exclude", "b", "--include", "c"},
		cmd,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmd.Include) != 2 {
		t.Fatalf("expected 2 includes, got %d: %v", len(cmd.Include), cmd.Include)
	}

	if cmd.Include[0].Value != "a" || cmd.Include[0].Position != 0 {
		t.Fatalf("expected include[0]={a,0}, got %+v", cmd.Include[0])
	}

	if cmd.Include[1].Value != "c" || cmd.Include[1].Position != 2 {
		t.Fatalf("expected include[1]={c,2}, got %+v", cmd.Include[1])
	}

	if len(cmd.Exclude) != 1 {
		t.Fatalf("expected 1 exclude, got %d: %v", len(cmd.Exclude), cmd.Exclude)
	}

	if cmd.Exclude[0].Value != "b" || cmd.Exclude[0].Position != 1 {
		t.Fatalf("expected exclude[0]={b,1}, got %+v", cmd.Exclude[0])
	}
}

func TestMapFlags_InvalidFormat(t *testing.T) {
	cmd := &MapStringStringCmd{}

	_, err := targ.Execute([]string{"app", "--labels", "invalid"}, cmd)
	if err == nil {
		t.Fatal("expected error for invalid map format")
	}
}

func TestMapFlags_OverwriteKey(t *testing.T) {
	cmd := &MapStringStringCmd{}

	_, err := targ.Execute([]string{"app", "--labels", "env=dev", "--labels", "env=prod"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Labels["env"] != "prod" {
		t.Fatalf("expected env=prod (overwritten), got %q", cmd.Labels["env"])
	}
}

func TestMapFlags_StringInt(t *testing.T) {
	cmd := &MapStringIntCmd{}

	_, err := targ.Execute([]string{"app", "--ports", "http=80", "--ports", "https=443"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmd.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d: %v", len(cmd.Ports), cmd.Ports)
	}

	if cmd.Ports["http"] != 80 {
		t.Fatalf("expected http=80, got %d", cmd.Ports["http"])
	}

	if cmd.Ports["https"] != 443 {
		t.Fatalf("expected https=443, got %d", cmd.Ports["https"])
	}
}

func TestMapFlags_StringString(t *testing.T) {
	cmd := &MapStringStringCmd{}

	_, err := targ.Execute([]string{"app", "--labels", "env=prod", "--labels", "app=web"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmd.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d: %v", len(cmd.Labels), cmd.Labels)
	}

	if cmd.Labels["env"] != "prod" {
		t.Fatalf("expected env=prod, got %q", cmd.Labels["env"])
	}

	if cmd.Labels["app"] != "web" {
		t.Fatalf("expected app=web, got %q", cmd.Labels["app"])
	}
}

func TestMapFlags_ValueWithEquals(t *testing.T) {
	cmd := &MapStringStringCmd{}

	_, err := targ.Execute([]string{"app", "--labels", "equation=a=b"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Labels["equation"] != "a=b" {
		t.Fatalf("expected equation=a=b, got %q", cmd.Labels["equation"])
	}
}

func TestMultipleRoots_RoutesCorrectly(t *testing.T) {
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
	cmd := &RepeatedFlagsCmd{}

	_, err := targ.Execute([]string{"app", "--tags", "a", "--tags", "b", "--tags", "c"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmd.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(cmd.Tags), cmd.Tags)
	}

	if cmd.Tags[0] != "a" || cmd.Tags[1] != "b" || cmd.Tags[2] != "c" {
		t.Fatalf("unexpected tags order: %v", cmd.Tags)
	}
}

func TestRepeatedFlags_IntSlice(t *testing.T) {
	cmd := &RepeatedIntFlagsCmd{}

	_, err := targ.Execute([]string{"app", "--counts", "1", "--counts", "2", "--counts", "3"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmd.Counts) != 3 {
		t.Fatalf("expected 3 counts, got %d: %v", len(cmd.Counts), cmd.Counts)
	}

	if cmd.Counts[0] != 1 || cmd.Counts[1] != 2 || cmd.Counts[2] != 3 {
		t.Fatalf("unexpected counts: %v", cmd.Counts)
	}
}

func TestStringTarget_NoVars_ExecutesDirectly(t *testing.T) {
	// String target without variables should just execute
	target := targ.Targ("true").Name("simple-cmd")

	result, err := targ.Execute([]string{"app"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

func TestStringTarget_WithVars_EqualsValueSyntax(t *testing.T) {
	// Test --flag=value syntax
	target := targ.Targ("echo $name $port").Name("test-cmd")

	result, err := targ.Execute([]string{"app", "--name=hello", "--port=8080"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

func TestStringTarget_WithVars_InfersFlags(t *testing.T) {
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
	target := targ.Targ("echo $name $port").Name("test-cmd")

	// Missing --port should fail
	_, err := targ.Execute([]string{"app", "--name", "hello"}, target)
	if err == nil {
		t.Fatal("expected error for missing required flag")
	}
}

func TestStringTarget_WithVars_MixedSyntax(t *testing.T) {
	// Test mixing --flag=value and --flag value syntax
	target := targ.Targ("echo $name $port").Name("test-cmd")

	// Mix equals syntax with space syntax
	result, err := targ.Execute([]string{"app", "--name=hello", "--port", "8080"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

func TestStringTarget_WithVars_MultiRoot(t *testing.T) {
	// With multiple roots, need to specify command name
	target := targ.Targ("echo $name").Name("echo-cmd")
	other := targ.Targ(func() {}).Name("other")

	result, err := targ.Execute([]string{"app", "echo-cmd", "--name", "hello"}, target, other)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

func TestStringTarget_WithVars_ShortFlags(t *testing.T) {
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
	cmd := &TimeoutCmd{}

	_, err := targ.Execute([]string{"app", "--timeout=1s"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cmd.Called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_Exceeded(t *testing.T) {
	cmd := &SlowCmd{}

	_, err := targ.Execute([]string{"app", "--timeout", "10ms"}, cmd)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestTimeout_GlobalAndPerCommand(t *testing.T) {
	cmd := &TimeoutCmd{}

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "--timeout", "5s", "timeout-cmd", "--timeout", "1s"},
		targ.RunOptions{AllowDefault: false},
		cmd,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cmd.Called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_InvalidDuration(t *testing.T) {
	cmd := &TimeoutCmd{}

	_, err := targ.Execute([]string{"app", "--timeout", "invalid"}, cmd)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestTimeout_MultiCommandDifferentTimeouts(t *testing.T) {
	fast := &FastCmd{}
	slow := &SlowCmd{}

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "fast-cmd", "--timeout", "10ms", "slow-cmd", "--timeout", "2s"},
		targ.RunOptions{AllowDefault: false},
		fast, slow,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fast.Called {
		t.Fatal("expected fast command to be called")
	}

	if !slow.Called {
		t.Fatal("expected slow command to be called")
	}
}

func TestTimeout_NotExceeded(t *testing.T) {
	cmd := &TimeoutCmd{}

	_, err := targ.Execute([]string{"app", "--timeout", "1s"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cmd.Called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_PerCommand(t *testing.T) {
	cmd := &TimeoutCmd{}

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "timeout-cmd", "--timeout", "1s"},
		targ.RunOptions{AllowDefault: false},
		cmd,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cmd.Called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_PerCommandExceeded(t *testing.T) {
	cmd := &SlowCmd{}

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "slow-cmd", "--timeout", "10ms"},
		targ.RunOptions{AllowDefault: false},
		cmd,
	)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// unexported constants.
const (
	testNameAlice = "Alice"
)
