package targs

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Helper for tests
func parseCommand(f interface{}) (*CommandNode, error) {
	return parseStruct(f)
}

type MyCommandStruct struct {
	Name string
}

func (c *MyCommandStruct) Run() {}

func TestParseCommand(t *testing.T) {
	cmd, err := parseCommand(&MyCommandStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Name != "my-command-struct" {
		t.Errorf("expected Name to be 'my-command-struct', got '%s'", cmd.Name)
	}
}

func TestParseNilPointer(t *testing.T) {
	var cmd *MyCommandStruct
	if _, err := parseCommand(cmd); err == nil {
		t.Fatal("expected error for nil pointer target")
	}
}

type TestCmdStruct struct {
	Name   string
	Called bool
}

func (c *TestCmdStruct) Run() {
	c.Called = true
}

func TestExecuteCommand(t *testing.T) {
	cmdStruct := &TestCmdStruct{}

	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := []string{"--name", "Alice"}
	if err := cmd.execute(context.Background(), args); err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if !cmdStruct.Called {
		t.Fatal("function was not called")
	}

	if cmdStruct.Name != "Alice" {
		t.Errorf("expected Name='Alice', got '%s'", cmdStruct.Name)
	}
}

type CustomArgs struct {
	User   string `targs:"name=user_name"`
	Called bool
}

func (c *CustomArgs) Run() {
	c.Called = true
}

func TestCustomFlagName(t *testing.T) {
	cmdStruct := &CustomArgs{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// use --user_name instead of -user
	args := []string{"--user_name", "Bob"}
	if err := cmd.execute(context.Background(), args); err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if cmdStruct.User != "Bob" {
		t.Errorf("expected User='Bob', got '%s'", cmdStruct.User)
	}
}

type RequiredArgs struct {
	ID string `targs:"required"`
}

func (c *RequiredArgs) Run() {}

func TestRequiredFlag(t *testing.T) {
	cmd, _ := parseCommand(&RequiredArgs{})

	// Missing required flag should error
	if err := cmd.execute(context.Background(), []string{}); err == nil {
		t.Fatal("expected error for missing required flag")
	} else if !strings.Contains(err.Error(), "--id") {
		t.Fatalf("expected error to mention --id, got: %v", err)
	}
}

type EnvArgs struct {
	User string `targs:"env=TEST_USER"`
}

func (c *EnvArgs) Run() {}

func TestEnvVars(t *testing.T) {
	cmdStruct := &EnvArgs{}
	cmd, _ := parseCommand(cmdStruct)

	os.Setenv("TEST_USER", "EnvAlice")
	defer os.Unsetenv("TEST_USER")

	if err := cmd.execute(context.Background(), []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmdStruct.User != "EnvAlice" {
		t.Errorf("expected User='EnvAlice', got '%s'", cmdStruct.User)
	}
}

type DefaultArgs struct {
	Name    string `targs:"default=Alice"`
	Count   int    `targs:"default=42"`
	Enabled bool   `targs:"default=true"`
}

func (c *DefaultArgs) Run() {}

func TestStructDefaults(t *testing.T) {
	cmdStruct := &DefaultArgs{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmdStruct.Name != "Alice" {
		t.Errorf("expected Name='Alice', got '%s'", cmdStruct.Name)
	}
	if cmdStruct.Count != 42 {
		t.Errorf("expected Count=42, got %d", cmdStruct.Count)
	}
	if !cmdStruct.Enabled {
		t.Error("expected Enabled=true")
	}
}

type DefaultEnvArgs struct {
	Name  string `targs:"default=Alice,env=TEST_DEFAULT_NAME"`
	Count int    `targs:"default=42,env=TEST_DEFAULT_COUNT"`
	Flag  bool   `targs:"default=true,env=TEST_DEFAULT_FLAG"`
}

func (c *DefaultEnvArgs) Run() {}

func TestStructDefaults_EnvOverrides(t *testing.T) {
	cmdStruct := &DefaultEnvArgs{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	os.Setenv("TEST_DEFAULT_NAME", "Bob")
	os.Setenv("TEST_DEFAULT_COUNT", "7")
	os.Setenv("TEST_DEFAULT_FLAG", "false")
	defer func() {
		os.Unsetenv("TEST_DEFAULT_NAME")
		os.Unsetenv("TEST_DEFAULT_COUNT")
		os.Unsetenv("TEST_DEFAULT_FLAG")
	}()

	if err := cmd.execute(context.Background(), []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmdStruct.Name != "Bob" {
		t.Errorf("expected Name='Bob', got '%s'", cmdStruct.Name)
	}
	if cmdStruct.Count != 7 {
		t.Errorf("expected Count=7, got %d", cmdStruct.Count)
	}
	if cmdStruct.Flag {
		t.Error("expected Flag=false")
	}
}

func TestStructDefaults_InvalidEnvValues(t *testing.T) {
	cmdStruct := &DefaultEnvArgs{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	os.Setenv("TEST_DEFAULT_COUNT", "nope")
	defer os.Unsetenv("TEST_DEFAULT_COUNT")

	if err := cmd.execute(context.Background(), []string{}); err == nil {
		t.Fatal("expected error for invalid env value")
	} else if !strings.Contains(err.Error(), "TEST_DEFAULT_COUNT") {
		t.Fatalf("expected error to mention env var, got: %v", err)
	}
}

type UnexportedFlag struct {
	hidden string `targs:"flag"`
}

func (c *UnexportedFlag) Run() {}

func TestUnexportedFieldError(t *testing.T) {
	cmdStruct := &UnexportedFlag{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{}); err == nil {
		t.Fatal("expected error for unexported tagged field")
	}
}

type ErrorRunCmd struct{}

func (c *ErrorRunCmd) Run() error {
	return fmt.Errorf("boom")
}

func TestRunReturnsError(t *testing.T) {
	cmdStruct := &ErrorRunCmd{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{}); err == nil {
		t.Fatal("expected error from Run")
	}
}

func ErrorFunc() error {
	return fmt.Errorf("nope")
}

func TestFunctionReturnsError(t *testing.T) {
	node, err := parseTarget(ErrorFunc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := node.execute(context.Background(), []string{}); err == nil {
		t.Fatal("expected error from function command")
	}
}

type TextValue struct {
	Value string
}

func (t *TextValue) UnmarshalText(text []byte) error {
	t.Value = strings.ToUpper(string(text))
	return nil
}

type SetterValue struct {
	Value string
}

func (s *SetterValue) Set(value string) error {
	s.Value = value + "!"
	return nil
}

type CustomTypeArgs struct {
	Name TextValue   `targs:"flag"`
	Nick SetterValue `targs:"flag"`
	Pos  TextValue   `targs:"positional"`
}

func (c *CustomTypeArgs) Run() {}

func TestCustomTypesFromFlagsAndPositionals(t *testing.T) {
	cmdStruct := &CustomTypeArgs{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{"--name", "alice", "--nick", "bob", "pos"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmdStruct.Name.Value != "ALICE" {
		t.Fatalf("expected name to be set via UnmarshalText, got %q", cmdStruct.Name.Value)
	}
	if cmdStruct.Nick.Value != "bob!" {
		t.Fatalf("expected nick to be set via Set, got %q", cmdStruct.Nick.Value)
	}
	if cmdStruct.Pos.Value != "POS" {
		t.Fatalf("expected positional to be set via UnmarshalText, got %q", cmdStruct.Pos.Value)
	}
}

type DefaultTagArgs struct {
	Name  string `targs:"default=alice"`
	Count int    `targs:"default=2"`
	Flag  bool   `targs:"default=true"`
	Pos   string `targs:"positional,default=pos"`
}

func (d *DefaultTagArgs) Run() {}

func TestDefaultTags(t *testing.T) {
	cmdStruct := &DefaultTagArgs{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmdStruct.Name != "alice" {
		t.Fatalf("expected default name, got %q", cmdStruct.Name)
	}
	if cmdStruct.Count != 2 {
		t.Fatalf("expected default count 2, got %d", cmdStruct.Count)
	}
	if !cmdStruct.Flag {
		t.Fatal("expected default flag true")
	}
	if cmdStruct.Pos != "pos" {
		t.Fatalf("expected default positional, got %q", cmdStruct.Pos)
	}
}

type DefaultTagCustom struct {
	Name TextValue `targs:"default=alice"`
}

func (d *DefaultTagCustom) Run() {}

func TestDefaultTagCustomType(t *testing.T) {
	cmdStruct := &DefaultTagCustom{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmdStruct.Name.Value != "ALICE" {
		t.Fatalf("expected custom default to be applied, got %q", cmdStruct.Name.Value)
	}
}

type NonZeroRoot struct {
	Name string `targs:"flag"`
}

func (n *NonZeroRoot) Run() {}

func TestNonZeroRootErrors(t *testing.T) {
	_, err := parseCommand(&NonZeroRoot{Name: "set"})
	if err == nil {
		t.Fatal("expected error for non-zero root value")
	}
}

type RootWithPresetSub struct {
	Sub *SubCmd `targs:"subcommand"`
}

func (r *RootWithPresetSub) Run() {}

func TestNonZeroSubcommandErrors(t *testing.T) {
	_, err := parseCommand(&RootWithPresetSub{Sub: &SubCmd{}})
	if err == nil {
		t.Fatal("expected error for non-zero subcommand field")
	}
}

type ContextRunCmd struct {
	Seen bool
}

func (c *ContextRunCmd) Run(ctx context.Context) {
	if ctx != nil {
		c.Seen = true
	}
}

func TestRunWithContext(t *testing.T) {
	cmdStruct := &ContextRunCmd{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmdStruct.Seen {
		t.Fatal("expected Run(ctx) to be called with context")
	}
}

type SubCmd struct {
	Verbose bool
	Called  bool
}

func (s *SubCmd) Run() {
	s.Called = true
}

type ParentCmd struct {
	// Name should default to "sub" from field name
	Sub *SubCmd `targs:"subcommand"`
	// Name should be "custom" from tag
	Custom *SubCmd `targs:"subcommand=custom"`
}

func TestSubcommands(t *testing.T) {
	cmdStruct := &ParentCmd{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check if subcommand exists
	if len(cmd.Subcommands) != 2 {
		t.Fatalf("expected 2 subcommands, got %d", len(cmd.Subcommands))
	}

	if _, ok := cmd.Subcommands["sub"]; !ok {
		t.Errorf("expected subcommand 'sub'")
	}
	if _, ok := cmd.Subcommands["custom"]; !ok {
		t.Errorf("expected subcommand 'custom'")
	}

	// Execute subcommand "sub"
	args := []string{"sub", "--verbose"}
	if err := cmd.execute(context.Background(), args); err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Execute subcommand "custom"
	args2 := []string{"custom", "--verbose"}
	if err := cmd.execute(context.Background(), args2); err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

type ShortFlagCmd struct {
	Name string `targs:"flag,short=n"`
	Age  int    `targs:"flag,short=a"`
}

func (c *ShortFlagCmd) Run() {}

func TestShortFlags(t *testing.T) {
	cmdStruct := &ShortFlagCmd{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := []string{"-n", "Alice", "-a", "30"}
	if err := cmd.execute(context.Background(), args); err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if cmdStruct.Name != "Alice" {
		t.Errorf("expected Name='Alice', got '%s'", cmdStruct.Name)
	}
	if cmdStruct.Age != 30 {
		t.Errorf("expected Age=30, got %d", cmdStruct.Age)
	}
}

type ShortBoolCmd struct {
	Verbose bool `targs:"flag,short=v"`
	Force   bool `targs:"flag,short=f"`
}

func (c *ShortBoolCmd) Run() {}

func TestShortFlagGroups(t *testing.T) {
	cmdStruct := &ShortBoolCmd{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{"-vf"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmdStruct.Verbose || !cmdStruct.Force {
		t.Fatalf("expected both flags to be set, got verbose=%v force=%v", cmdStruct.Verbose, cmdStruct.Force)
	}
}

type ShortMixedCmd struct {
	Verbose bool   `targs:"flag,short=v"`
	Name    string `targs:"flag,short=n"`
}

func (c *ShortMixedCmd) Run() {}

func TestShortFlagGroupsRejectValueFlags(t *testing.T) {
	cmdStruct := &ShortMixedCmd{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{"-vn"}); err == nil {
		t.Fatal("expected error for grouped short flag with value")
	}
}

func TestLongFlagsRequireDoubleDash(t *testing.T) {
	cmdStruct := &ShortFlagCmd{}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{"-name", "Alice"}); err == nil {
		t.Fatal("expected error for single-dash long flag")
	} else if !strings.Contains(err.Error(), "long flags must use --name") {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{"-name=Alice"}); err == nil {
		t.Fatal("expected error for single-dash long flag with equals")
	} else if !strings.Contains(err.Error(), "long flags must use --name") {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute(context.Background(), []string{"--name", "Alice", "--age", "30"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmdStruct.Name != "Alice" || cmdStruct.Age != 30 {
		t.Fatalf("expected parsed flags, got name=%q age=%d", cmdStruct.Name, cmdStruct.Age)
	}
}

type PositionalArgs struct {
	Src string `targs:"positional"`
	Dst string `targs:"positional"`
}

func (c *PositionalArgs) Run() {}

func TestPositionalArgs(t *testing.T) {
	cmdStruct := &PositionalArgs{}
	cmd, _ := parseCommand(cmdStruct)

	if err := cmd.execute(context.Background(), []string{"source.txt", "dest.txt"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmdStruct.Src != "source.txt" {
		t.Errorf("expected Src='source.txt', got '%s'", cmdStruct.Src)
	}
	if cmdStruct.Dst != "dest.txt" {
		t.Errorf("expected Dst='dest.txt', got '%s'", cmdStruct.Dst)
	}
}

func TestUsageLine_NoSubcommandWithRequiredPositional(t *testing.T) {
	type MoveCmd struct {
		File   string `targs:"flag,short=f"`
		Status string `targs:"flag,required,short=s"`
		ID     int    `targs:"positional,required"`
	}
	cmd, err := parseCommand(&MoveCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usage := buildUsageLine(cmd)
	if strings.Contains(usage, "[subcommand]") {
		t.Fatalf("did not expect subcommand in usage: %s", usage)
	}
	if !strings.Contains(usage, "{-f|--file} <string>") {
		t.Fatalf("expected file flag in usage: %s", usage)
	}
	if !strings.Contains(usage, "{-s|--status} <string>") {
		t.Fatalf("expected status flag in usage: %s", usage)
	}
	if !strings.HasSuffix(usage, "ID") {
		t.Fatalf("expected ID positional at end: %s", usage)
	}
}

func TestPlaceholderTagInUsage(t *testing.T) {
	type PlaceholderCmd struct {
		File string `targs:"flag,short=f,placeholder=FILE"`
		Out  string `targs:"positional,placeholder=DEST"`
	}
	cmd, err := parseCommand(&PlaceholderCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usage := buildUsageLine(cmd)
	if !strings.Contains(usage, "{-f|--file} FILE") {
		t.Fatalf("expected flag placeholder in usage: %s", usage)
	}
	if !strings.HasSuffix(usage, "[DEST]") {
		t.Fatalf("expected positional placeholder in usage: %s", usage)
	}
}

func TestPositionalEnumUsage(t *testing.T) {
	type EnumPositional struct {
		Mode string `targs:"positional,enum=dev|prod"`
	}
	cmd, err := parseCommand(&EnumPositional{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usage := buildUsageLine(cmd)
	if !strings.Contains(usage, "{dev|prod}") {
		t.Fatalf("expected enum positional placeholder, got: %s", usage)
	}
}

var hookLog []string

type HookRoot struct {
	Verbose bool     `targs:"flag,short=v"`
	Child   *HookCmd `targs:"subcommand"`
}

func (h *HookRoot) PersistentBefore() {
	hookLog = append(hookLog, "root:before")
}

func (h *HookRoot) PersistentAfter() {
	hookLog = append(hookLog, "root:after")
}

type HookCmd struct {
	Name string `targs:"flag"`
}

func (h *HookCmd) PersistentBefore() {
	hookLog = append(hookLog, "child:before")
}

func (h *HookCmd) PersistentAfter() {
	hookLog = append(hookLog, "child:after")
}

func (h *HookCmd) Run() {
	hookLog = append(hookLog, "child:run")
}

func TestPersistentFlagsInherited(t *testing.T) {
	hookLog = nil
	root := &HookRoot{}
	cmd, err := parseCommand(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cmd.execute(context.Background(), []string{"child", "--verbose", "--name", "ok"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !root.Verbose {
		t.Fatal("expected root verbose flag to be set from subcommand args")
	}
}

func TestPersistentHooksOrder(t *testing.T) {
	hookLog = nil
	root := &HookRoot{}
	cmd, err := parseCommand(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cmd.execute(context.Background(), []string{"child"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"root:before", "child:before", "child:run", "child:after", "root:after"}
	if !reflect.DeepEqual(hookLog, want) {
		t.Fatalf("unexpected hook order: %v", hookLog)
	}
}

func TestHelpIncludesInheritedFlags(t *testing.T) {
	root := &HookRoot{}
	cmd, err := parseCommand(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	child := cmd.Subcommands["child"]
	if child == nil {
		t.Fatal("expected child subcommand")
	}
	out := captureStdout(t, func() {
		printCommandHelp(child)
	})
	if !strings.Contains(out, "--verbose") {
		t.Fatalf("expected inherited flag in help, got: %q", out)
	}
}

func TestCompletionIncludesInheritedFlags(t *testing.T) {
	root := &HookRoot{}
	cmd, err := parseCommand(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := captureStdout(t, func() {
		doCompletion([]*CommandNode{cmd}, "app child --")
	})
	if !strings.Contains(out, "--verbose") {
		t.Fatalf("expected inherited flag in completion, got: %q", out)
	}
}

type ConflictRoot struct {
	Flag string         `targs:"flag"`
	Sub  *ConflictChild `targs:"subcommand"`
}

type ConflictChild struct {
	Flag string `targs:"flag"`
}

func (c *ConflictChild) Run() {}

func TestPersistentFlagConflicts(t *testing.T) {
	root := &ConflictRoot{}
	cmd, err := parseCommand(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cmd.execute(context.Background(), []string{"sub", "--flag", "ok"}); err == nil {
		t.Fatal("expected error for conflicting flag names")
	}
}

type RequiredPositionals struct {
	Src string `targs:"positional,required"`
	Dst string `targs:"positional"`
}

func (c *RequiredPositionals) Run() {}

func TestRequiredPositionals(t *testing.T) {
	cmdStruct := &RequiredPositionals{}
	cmd, _ := parseCommand(cmdStruct)

	if err := cmd.execute(context.Background(), []string{}); err == nil {
		t.Fatal("expected error for missing required positional")
	} else if !strings.Contains(err.Error(), "Src") {
		t.Fatalf("expected error to mention Src, got: %v", err)
	}
}

type RequiredShortFlag struct {
	Name string `targs:"required,short=n"`
}

func (c *RequiredShortFlag) Run() {}

func TestRequiredShortFlagErrorIncludesShort(t *testing.T) {
	cmdStruct := &RequiredShortFlag{}
	cmd, _ := parseCommand(cmdStruct)

	if err := cmd.execute(context.Background(), []string{}); err == nil {
		t.Fatal("expected error for missing required flag")
	} else if !strings.Contains(err.Error(), "--name") || !strings.Contains(err.Error(), "-n") {
		t.Fatalf("expected error to mention --name and -n, got: %v", err)
	}
}

type RequiredEnvFlag struct {
	Name string `targs:"required,env=TEST_REQUIRED"`
}

func (c *RequiredEnvFlag) Run() {}

func TestRequiredEnvFlagEmptyDoesNotSatisfy(t *testing.T) {
	cmdStruct := &RequiredEnvFlag{}
	cmd, _ := parseCommand(cmdStruct)

	os.Setenv("TEST_REQUIRED", "")
	defer os.Unsetenv("TEST_REQUIRED")

	if err := cmd.execute(context.Background(), []string{}); err == nil {
		t.Fatal("expected error for missing required flag when env is empty")
	}
}

// --- Discovery Tests ---

type RootA struct {
	Sub *ChildB `targs:"subcommand"`
}
type ChildB struct{}
type RootC struct{}

func TestDetectRootCommands(t *testing.T) {
	candidates := []interface{}{
		&RootA{},
		&ChildB{},
		&RootC{},
	}

	roots := DetectRootCommands(candidates...)

	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}

	// RootA and RootC should be present. ChildB should be filtered out.
	hasA := false
	hasC := false
	hasB := false

	for _, r := range roots {
		switch r.(type) {
		case *RootA:
			hasA = true
		case *RootC:
			hasC = true
		case *ChildB:
			hasB = true
		}
	}

	if !hasA {
		t.Error("expected RootA to be detected")
	}
	if !hasC {
		t.Error("expected RootC to be detected")
	}
	if hasB {
		t.Error("ChildB should have been filtered out")
	}
}

type MetaOverrideCmd struct{}

func (m *MetaOverrideCmd) Run() {}

func (m *MetaOverrideCmd) Name() string {
	return "CustomName"
}

func (m MetaOverrideCmd) Description() string {
	return "Custom description."
}

func TestCommandMetaOverrides(t *testing.T) {
	node, err := parseStruct(&MetaOverrideCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Name != "custom-name" {
		t.Fatalf("expected command name custom-name, got %q", node.Name)
	}
	if node.Description != "Custom description." {
		t.Fatalf("expected description override, got %q", node.Description)
	}
}
