package targ

// internal_test.go contains whitebox tests that require access to unexported
// symbols. These test internal implementation details that cannot be easily
// tested through the public Execute() API.
//
// Most tests should be blackbox tests in the test/ directory.

import (
	"strings"
	"testing"
)

// --- Parsing Edge Cases ---

func TestParseNilPointer(t *testing.T) {
	var cmd *MyCommandStruct
	if _, err := parseStruct(cmd); err == nil {
		t.Fatal("expected error for nil pointer target")
	}
}

type MyCommandStruct struct {
	Name string
}

func (c *MyCommandStruct) Run() {}

type UnexportedFlag struct {
	hidden string `targ:"flag"`
}

func (c *UnexportedFlag) Run() {}

func TestUnexportedFieldError(t *testing.T) {
	cmdStruct := &UnexportedFlag{}
	cmd, err := parseStruct(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Error occurs at execution time, not parse time
	if err := cmd.execute(nil, []string{}, RunOptions{}); err == nil {
		t.Fatal("expected error for unexported tagged field")
	}
}

type NonZeroRoot struct {
	Name string `targ:"flag"`
}

func (n *NonZeroRoot) Run() {}

func TestNonZeroRootErrors(t *testing.T) {
	_, err := parseStruct(&NonZeroRoot{Name: "set"})
	if err == nil {
		t.Fatal("expected error for non-zero root value")
	}
}

type RootWithPresetSub struct {
	Sub *SubCmdInternal `targ:"subcommand"`
}

func (r *RootWithPresetSub) Run() {}

type SubCmdInternal struct{}

func (s *SubCmdInternal) Run() {}

func TestNonZeroSubcommandErrors(t *testing.T) {
	_, err := parseStruct(&RootWithPresetSub{Sub: &SubCmdInternal{}})
	if err == nil {
		t.Fatal("expected error for non-zero subcommand field")
	}
}

// --- Usage Line Formatting ---

func TestUsageLine_NoSubcommandWithRequiredPositional(t *testing.T) {
	type MoveCmd struct {
		File   string `targ:"flag,short=f"`
		Status string `targ:"flag,required,short=s"`
		ID     int    `targ:"positional,required"`
	}
	cmd, err := parseStruct(&MoveCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usage, err := buildUsageLine(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(usage, "[subcommand]") {
		t.Fatalf("did not expect subcommand in usage: %s", usage)
	}
	if !strings.Contains(usage, "{-s|--status}") {
		t.Fatalf("expected status flag in usage: %s", usage)
	}
	if !strings.HasSuffix(usage, "ID") {
		t.Fatalf("expected ID positional at end: %s", usage)
	}
}

func TestPlaceholderTagInUsage(t *testing.T) {
	type PlaceholderCmd struct {
		File string `targ:"flag,short=f,placeholder=FILE"`
		Out  string `targ:"positional,placeholder=DEST"`
	}
	cmd, err := parseStruct(&PlaceholderCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usage, err := buildUsageLine(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(usage, "{-f|--file} FILE") {
		t.Fatalf("expected flag placeholder in usage: %s", usage)
	}
	if !strings.HasSuffix(usage, "[DEST]") {
		t.Fatalf("expected positional placeholder in usage: %s", usage)
	}
}

func TestPositionalEnumUsage(t *testing.T) {
	type EnumPositional struct {
		Mode string `targ:"positional,enum=dev|prod"`
	}
	cmd, err := parseStruct(&EnumPositional{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usage, err := buildUsageLine(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(usage, "{dev|prod}") {
		t.Fatalf("expected enum positional placeholder, got: %s", usage)
	}
}

// --- Command Metadata ---

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

// --- Flag Conflict Detection ---

type ConflictRoot struct {
	Flag string          `targ:"flag"`
	Sub  *ConflictChild2 `targ:"subcommand"`
}

type ConflictChild2 struct {
	Flag string `targ:"flag"`
}

func (c *ConflictChild2) Run() {}

func TestPersistentFlagConflicts(t *testing.T) {
	root := &ConflictRoot{}
	cmd, err := parseStruct(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cmd.execute(nil, []string{"sub", "--flag", "ok"}, RunOptions{}); err == nil {
		t.Fatal("expected error for conflicting flag names")
	}
}

// --- Helpers for other test files ---

// parseCommand is a helper used by completion_test.go
func parseCommand(f interface{}) (*commandNode, error) {
	return parseStruct(f)
}

// TestCmdStruct is used by run_function_test.go
type TestCmdStruct struct {
	Name   string
	Called bool
}

func (c *TestCmdStruct) Run() {
	c.Called = true
}
