package commander

import (
	"testing"
	"os"
	"fmt"
)

// Helper for tests
func parseCommand(f interface{}) (*commandDefinition, error) {
	cmds, err := parseTarget(f)
	if err != nil {
		return nil, err
	}
	if len(cmds) == 0 {
		return nil, fmt.Errorf("no commands found")
	}
	return cmds[0], nil
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

	if cmd.Args.Name() != "MyCommandStruct" {
		t.Errorf("expected Args to be 'MyCommandStruct', got '%s'", cmd.Args.Name())
	}
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

type TestCmdStruct struct {
	Name string
	Called bool
}
func (c *TestCmdStruct) Run() {
	c.Called = true
}

type CustomArgs struct {
	User string `commander:"name=user_name"`
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

	// Missing required flag
	if err := cmd.execute([]string{}); err == nil {
		t.Error("expected error for missing required flag, got nil")
	} else if err.Error() != "required flag -id is missing" {
		t.Errorf("unexpected error message: %v", err)
	}

	// Provided required flag
	if err := cmd.execute([]string{"-id", "123"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

type EnvArgs struct {
	User string `commander:"required,env=TEST_USER"`
	Age  int    `commander:"env=TEST_AGE"`
	Rich bool   `commander:"env=TEST_RICH"`
}
func (c *EnvArgs) Run() {}

func TestEnvVars(t *testing.T) {
	cmdStruct := &EnvArgs{}
	cmd, _ := parseCommand(cmdStruct)

	// Set env vars
	os.Setenv("TEST_USER", "EnvAlice")
	os.Setenv("TEST_AGE", "42")
	os.Setenv("TEST_RICH", "true")
	defer func() {
		os.Unsetenv("TEST_USER")
		os.Unsetenv("TEST_AGE")
		os.Unsetenv("TEST_RICH")
	}()

	// No args provided, should satisfy required from env
	if err := cmd.execute([]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmdStruct.User != "EnvAlice" {
		t.Errorf("expected User='EnvAlice', got '%s'", cmdStruct.User)
	}
	if cmdStruct.Age != 42 {
		t.Errorf("expected Age=42, got %d", cmdStruct.Age)
	}
	if cmdStruct.Rich != true {
		t.Errorf("expected Rich=true, got %v", cmdStruct.Rich)
	}

	// CLI overrides Env
	if err := cmd.execute([]string{"-user", "CliBob"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmdStruct.User != "CliBob" {
		t.Errorf("expected User='CliBob', got '%s'", cmdStruct.User)
	}
}

type RemoteArgs struct {
	Verbose bool
}

type AddArgs struct {
	Url string
}

type Remote struct {}

func (r Remote) Add(args AddArgs) {}

func TestSubcommands(t *testing.T) {
	cmds, err := parseTarget(Remote{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}

	if cmds[0].Name != "remote add" {
		t.Errorf("expected Name='remote add', got '%s'", cmds[0].Name)
	}
}

type RunCmd struct {}
type RunArgs struct {}

func (r *RunCmd) Run(args RunArgs) {}

func TestRunSubcommand(t *testing.T) {
	cmds, err := parseTarget(&RunCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}

	// Should be "run-cmd run", not "run-cmd"
	if cmds[0].Name != "run-cmd run" {
		t.Errorf("expected Name='run-cmd run', got '%s'", cmds[0].Name)
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

type VariadicArgs struct {
	Cmd  string   `commander:"positional"`
	Args []string `commander:"positional"`
}
func (c *VariadicArgs) Run() {}

func TestVariadicArgs(t *testing.T) {
	cmdStruct := &VariadicArgs{}
	cmd, _ := parseCommand(cmdStruct)

	if err := cmd.execute([]string{"echo", "hello", "world"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmdStruct.Cmd != "echo" {
		t.Errorf("expected Cmd='echo', got '%s'", cmdStruct.Cmd)
	}
	if len(cmdStruct.Args) != 2 {
		t.Fatalf("expected 2 variadic args, got %d", len(cmdStruct.Args))
	}
	if cmdStruct.Args[0] != "hello" || cmdStruct.Args[1] != "world" {
		t.Errorf("unexpected variadic args: %v", cmdStruct.Args)
	}
}
