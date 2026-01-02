package commander

import (
	"context"
	"fmt"
	"os"
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

	args := []string{"-name", "Alice"}
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
	User   string `commander:"name=user_name"`
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

	// use -user_name instead of -user
	args := []string{"-user_name", "Bob"}
	if err := cmd.execute(context.Background(), args); err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if cmdStruct.User != "Bob" {
		t.Errorf("expected User='Bob', got '%s'", cmdStruct.User)
	}
}

type RequiredArgs struct {
	ID string `commander:"required"`
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
	User string `commander:"env=TEST_USER"`
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
	Name    string `commander:"default=Alice"`
	Count   int    `commander:"default=42"`
	Enabled bool   `commander:"default=true"`
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
	Name  string `commander:"default=Alice,env=TEST_DEFAULT_NAME"`
	Count int    `commander:"default=42,env=TEST_DEFAULT_COUNT"`
	Flag  bool   `commander:"default=true,env=TEST_DEFAULT_FLAG"`
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

type UnexportedFlag struct {
	hidden string `commander:"flag"`
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
	Name TextValue   `commander:"flag"`
	Nick SetterValue `commander:"flag"`
	Pos  TextValue   `commander:"positional"`
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
	Name  string `commander:"default=alice"`
	Count int    `commander:"default=2"`
	Flag  bool   `commander:"default=true"`
	Pos   string `commander:"positional,default=pos"`
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
	Name TextValue `commander:"default=alice"`
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
	Name string `commander:"flag"`
}

func (n *NonZeroRoot) Run() {}

func TestNonZeroRootErrors(t *testing.T) {
	_, err := parseCommand(&NonZeroRoot{Name: "set"})
	if err == nil {
		t.Fatal("expected error for non-zero root value")
	}
}

type RootWithPresetSub struct {
	Sub *SubCmd `commander:"subcommand"`
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
	Sub *SubCmd `commander:"subcommand"`
	// Name should be "custom" from tag
	Custom *SubCmd `commander:"subcommand=custom"`
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
	args := []string{"sub", "-verbose"}
	if err := cmd.execute(context.Background(), args); err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Execute subcommand "custom"
	args2 := []string{"custom", "-verbose"}
	if err := cmd.execute(context.Background(), args2); err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

type ShortFlagCmd struct {
	Name string `commander:"flag,short=n"`
	Age  int    `commander:"flag,short=a"`
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

type PositionalArgs struct {
	Src string `commander:"positional"`
	Dst string `commander:"positional"`
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

type RequiredPositionals struct {
	Src string `commander:"positional,required"`
	Dst string `commander:"positional"`
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
	Name string `commander:"required,short=n"`
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
	Name string `commander:"required,env=TEST_REQUIRED"`
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
	Sub *ChildB `commander:"subcommand"`
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

func (m *MetaOverrideCmd) CommandName() string {
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
