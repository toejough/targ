package commander

import (
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
	if err := cmd.execute(args); err != nil {
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
	if err := cmd.execute(args); err != nil {
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
	if err := cmd.execute([]string{}); err == nil {
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

	if err := cmd.execute([]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmdStruct.User != "EnvAlice" {
		t.Errorf("expected User='EnvAlice', got '%s'", cmdStruct.User)
	}
}

type DefaultArgs struct {
	Name    string
	Count   int
	Enabled bool
}

func (c *DefaultArgs) Run() {}

func TestStructDefaults(t *testing.T) {
	cmdStruct := &DefaultArgs{
		Name:    "Alice",
		Count:   42,
		Enabled: true,
	}
	cmd, err := parseCommand(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cmd.execute([]string{}); err != nil {
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
	Name  string `commander:"env=TEST_DEFAULT_NAME"`
	Count int    `commander:"env=TEST_DEFAULT_COUNT"`
	Flag  bool   `commander:"env=TEST_DEFAULT_FLAG"`
}

func (c *DefaultEnvArgs) Run() {}

func TestStructDefaults_EnvOverrides(t *testing.T) {
	cmdStruct := &DefaultEnvArgs{
		Name:  "Alice",
		Count: 42,
		Flag:  true,
	}
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

	if err := cmd.execute([]string{}); err != nil {
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
	if err := cmd.execute(args); err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Execute subcommand "custom"
	args2 := []string{"custom", "-verbose"}
	if err := cmd.execute(args2); err != nil {
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
	if err := cmd.execute(args); err != nil {
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

	if err := cmd.execute([]string{"source.txt", "dest.txt"}); err != nil {
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

	if err := cmd.execute([]string{}); err == nil {
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

	if err := cmd.execute([]string{}); err == nil {
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

	if err := cmd.execute([]string{}); err == nil {
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
