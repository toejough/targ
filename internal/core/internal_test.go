package core

// internal_test.go contains whitebox tests that require access to unexported
// symbols. These test internal implementation details that cannot be easily
// tested through the public Execute() API.
//
// Most tests should be blackbox tests in the test/ directory.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

type ConflictChild2 struct {
	Flag string `targ:"flag"`
}

func (c *ConflictChild2) Run() {}

// --- Flag Conflict Detection ---

type ConflictRoot struct {
	Flag string          `targ:"flag"`
	Sub  *ConflictChild2 `targ:"subcommand"`
}

// --- Command Metadata ---

type MetaOverrideCmd struct{}

func (m MetaOverrideCmd) Description() string {
	return "Custom description."
}

func (m *MetaOverrideCmd) Name() string {
	return "CustomName"
}

func (m *MetaOverrideCmd) Run() {}

type MyCommandStruct struct {
	Name string
}

func (c *MyCommandStruct) Run() {}

type NonZeroRoot struct {
	Name string `targ:"flag"`
}

func (n *NonZeroRoot) Run() {}

type RootWithPresetSub struct {
	Sub *SubCmdInternal `targ:"subcommand"`
}

func (r *RootWithPresetSub) Run() {}

type SubCmdInternal struct{}

func (s *SubCmdInternal) Run() {}

// TestCmdStruct is used by run_function_test.go
type TestCmdStruct struct {
	Name   string
	Called bool
}

func (c *TestCmdStruct) Run() {
	c.Called = true
}

type UnexportedFlag struct {
	hidden string `targ:"flag"` //nolint:unused // intentionally unexported for testing error handling
}

func (c *UnexportedFlag) Run() {}

// --- camelToKebab ---

func TestCamelToKebab(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"FooBar", "foo-bar"},
		{"CI", "ci"},
		{"CLI", "cli"},
		{"APIServer", "api-server"},
		{"FooAPI", "foo-api"},
		{"getHTTPResponse", "get-http-response"},
		{"HTTPSConnection", "https-connection"},
		{"SimpleTest", "simple-test"},
		{"ABC", "abc"},
		{"ABCdef", "ab-cdef"},
		{"Test", "test"},
		{"test", "test"},
		{"", ""},
	}
	for _, tt := range tests {
		got := camelToKebab(tt.input)
		if got != tt.want {
			t.Errorf("camelToKebab(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
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

func TestNonZeroRootErrors(t *testing.T) {
	_, err := parseStruct(&NonZeroRoot{Name: "set"})
	if err == nil {
		t.Fatal("expected error for non-zero root value")
	}
}

func TestNonZeroSubcommandErrors(t *testing.T) {
	_, err := parseStruct(&RootWithPresetSub{Sub: &SubCmdInternal{}})
	if err == nil {
		t.Fatal("expected error for non-zero subcommand field")
	}
}

// --- Parsing Edge Cases ---

func TestParseNilPointer(t *testing.T) {
	var cmd *MyCommandStruct
	if _, err := parseStruct(cmd); err == nil {
		t.Fatal("expected error for nil pointer target")
	}
}

func TestPersistentFlagConflicts(t *testing.T) {
	root := &ConflictRoot{}
	cmd, err := parseStruct(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := cmd.execute(context.TODO(), []string{"sub", "--flag", "ok"}, RunOptions{}); err == nil {
		t.Fatal("expected error for conflicting flag names")
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

func TestUnexportedFieldError(t *testing.T) {
	cmdStruct := &UnexportedFlag{}
	cmd, err := parseStruct(cmdStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Error occurs at execution time, not parse time
	if err := cmd.execute(context.TODO(), []string{}, RunOptions{}); err == nil {
		t.Fatal("expected error for unexported tagged field")
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

// --- detectShell tests ---

func TestDetectShell_KnownShells(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		shell    string
		expected string
	}{
		{"/bin/bash", "bash"},
		{"/usr/bin/zsh", "zsh"},
		{"/usr/local/bin/fish", "fish"},
		{"bash", "bash"},
		{"zsh", "zsh"},
		{"fish", "fish"},
		{"/opt/homebrew/bin/bash", "bash"},
		{"C:\\Program Files\\Git\\bin\\bash", "bash"},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			t.Setenv("SHELL", tt.shell)
			g.Expect(detectShell()).To(Equal(tt.expected))
		})
	}
}

func TestDetectShell_UnknownOrEmpty(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name  string
		shell string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"sh", "/bin/sh"},
		{"tcsh", "/usr/bin/tcsh"},
		{"ksh", "/bin/ksh"},
		{"unknown", "/custom/shell"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SHELL", tt.shell)
			g.Expect(detectShell()).To(BeEmpty())
		})
	}
}

func TestDetectShell_Property_KnownShellsAlwaysDetected(t *testing.T) {
	knownShells := []string{"bash", "zsh", "fish"}

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)
		// Generate a random path prefix
		prefix := rapid.StringMatching(`^(/[a-z]+)*$`).Draw(rt, "prefix")
		shell := rapid.SampledFrom(knownShells).Draw(rt, "shell")
		fullPath := prefix + "/" + shell

		// Test the shell extraction logic directly
		result := extractShellName(fullPath)
		g.Expect(result).To(Equal(shell), "path: %s", fullPath)
	})
}

// extractShellName extracts and validates the shell name from a path.
// This is the testable core logic of detectShell().
func extractShellName(shell string) string {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return ""
	}
	base := shell
	if idx := strings.LastIndex(base, "/"); idx != -1 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndex(base, "\\"); idx != -1 {
		base = base[idx+1:]
	}
	switch base {
	case "bash", "zsh", "fish":
		return base
	default:
		return ""
	}
}

// --- collectCommands tests ---

func TestCollectCommands_SingleRoot(t *testing.T) {
	g := NewWithT(t)

	node := &commandNode{
		Name:        "build",
		Description: "Build the project",
	}

	commands := make([]listCommandInfo, 0)
	collectCommands(node, "", &commands)

	g.Expect(commands).To(HaveLen(1))
	if len(commands) > 0 {
		g.Expect(commands[0].Name).To(Equal("build"))
		g.Expect(commands[0].Description).To(Equal("Build the project"))
	}
}

func TestCollectCommands_WithSubcommands(t *testing.T) {
	g := NewWithT(t)

	node := &commandNode{
		Name:        "app",
		Description: "Main application",
		Subcommands: map[string]*commandNode{
			"run": {
				Name:        "run",
				Description: "Run the app",
			},
			"build": {
				Name:        "build",
				Description: "Build the app",
			},
		},
	}

	commands := make([]listCommandInfo, 0)
	collectCommands(node, "", &commands)

	g.Expect(commands).To(HaveLen(3))

	// Find commands by name
	names := make(map[string]string)
	for _, cmd := range commands {
		names[cmd.Name] = cmd.Description
	}

	g.Expect(names).To(HaveKey("app"))
	g.Expect(names).To(HaveKey("app run"))
	g.Expect(names).To(HaveKey("app build"))
}

func TestCollectCommands_NestedSubcommands(t *testing.T) {
	g := NewWithT(t)

	node := &commandNode{
		Name: "root",
		Subcommands: map[string]*commandNode{
			"level1": {
				Name: "level1",
				Subcommands: map[string]*commandNode{
					"level2": {
						Name:        "level2",
						Description: "Deeply nested",
					},
				},
			},
		},
	}

	commands := make([]listCommandInfo, 0)
	collectCommands(node, "", &commands)

	g.Expect(commands).To(HaveLen(3))

	names := make([]string, len(commands))
	for i, cmd := range commands {
		names[i] = cmd.Name
	}

	g.Expect(names).To(ContainElements("root", "root level1", "root level1 level2"))
}

// --- doList tests ---

func TestDoList_SingleCommand(t *testing.T) {
	g := NewWithT(t)

	node := &commandNode{
		Name:        "build",
		Description: "Build the project",
	}

	output := captureStdout(t, func() {
		err := doList([]*commandNode{node})
		g.Expect(err).NotTo(HaveOccurred())
	})

	g.Expect(output).To(ContainSubstring(`"name": "build"`))
	g.Expect(output).To(ContainSubstring(`"description": "Build the project"`))
}

func TestDoList_MultipleCommands(t *testing.T) {
	g := NewWithT(t)

	nodes := []*commandNode{
		{Name: "build", Description: "Build it"},
		{Name: "test", Description: "Test it"},
	}

	output := captureStdout(t, func() {
		err := doList(nodes)
		g.Expect(err).NotTo(HaveOccurred())
	})

	g.Expect(output).To(ContainSubstring(`"name": "build"`))
	g.Expect(output).To(ContainSubstring(`"name": "test"`))
}

func TestDoList_IncludesSubcommands(t *testing.T) {
	g := NewWithT(t)

	node := &commandNode{
		Name: "app",
		Subcommands: map[string]*commandNode{
			"run": {Name: "run", Description: "Run it"},
		},
	}

	output := captureStdout(t, func() {
		err := doList([]*commandNode{node})
		g.Expect(err).NotTo(HaveOccurred())
	})

	g.Expect(output).To(ContainSubstring(`"name": "app"`))
	g.Expect(output).To(ContainSubstring(`"name": "app run"`))
}

// --- customSetter tests ---

// textUnmarshalerType implements encoding.TextUnmarshaler
type testTextUnmarshaler struct {
	value string
}

func (t *testTextUnmarshaler) UnmarshalText(text []byte) error {
	t.value = "unmarshaled:" + string(text)
	return nil
}

// stringSetterType implements Set(string) error
type testStringSetter struct {
	value string
}

func (t *testStringSetter) Set(s string) error {
	t.value = "set:" + s
	return nil
}

// plainType has no custom setter
type testPlainType struct {
	value string
}

func TestCustomSetter_TextUnmarshaler(t *testing.T) {
	g := NewWithT(t)

	var target testTextUnmarshaler
	val := reflect.ValueOf(&target).Elem()

	setter, ok := customSetter(val)
	g.Expect(ok).To(BeTrue(), "should find TextUnmarshaler")
	g.Expect(setter).NotTo(BeNil())

	err := setter("hello")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(target.value).To(Equal("unmarshaled:hello"))
}

func TestCustomSetter_StringSetter(t *testing.T) {
	g := NewWithT(t)

	var target testStringSetter
	val := reflect.ValueOf(&target).Elem()

	setter, ok := customSetter(val)
	g.Expect(ok).To(BeTrue(), "should find StringSetter")
	g.Expect(setter).NotTo(BeNil())

	err := setter("world")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(target.value).To(Equal("set:world"))
}

func TestCustomSetter_PlainType_NotFound(t *testing.T) {
	g := NewWithT(t)

	var target testPlainType
	val := reflect.ValueOf(&target).Elem()

	setter, ok := customSetter(val)
	g.Expect(ok).To(BeFalse(), "should not find setter for plain type")
	g.Expect(setter).To(BeNil())
}

func TestCustomSetter_NonAddressable(t *testing.T) {
	g := NewWithT(t)

	// Create a non-addressable value
	val := reflect.ValueOf(testPlainType{})

	setter, ok := customSetter(val)
	g.Expect(ok).To(BeFalse())
	g.Expect(setter).To(BeNil())
}

// --- binaryName tests ---

func TestBinaryName_FromOsArgs(t *testing.T) {
	g := NewWithT(t)

	// binaryName uses os.Args[0], so we test the logic it relies on
	// The function has a guard for empty os.Args, returns "targ" as default
	result := binaryName()
	g.Expect(result).NotTo(BeEmpty())
}

// --- extractTimeout tests ---

func TestExtractTimeout_NoTimeout(t *testing.T) {
	g := NewWithT(t)

	timeout, remaining, err := extractTimeout([]string{"cmd", "arg1", "arg2"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(timeout).To(BeZero())
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1", "arg2"}))
}

func TestExtractTimeout_WithEquals(t *testing.T) {
	g := NewWithT(t)

	timeout, remaining, err := extractTimeout([]string{"cmd", "--timeout=5m", "arg1"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(timeout.Minutes()).To(Equal(5.0))
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
}

func TestExtractTimeout_WithSeparateValue(t *testing.T) {
	g := NewWithT(t)

	timeout, remaining, err := extractTimeout([]string{"cmd", "--timeout", "10s", "arg1"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(timeout.Seconds()).To(Equal(10.0))
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
}

func TestExtractTimeout_MissingValue(t *testing.T) {
	g := NewWithT(t)

	_, _, err := extractTimeout([]string{"cmd", "--timeout"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires a duration"))
}

func TestExtractTimeout_InvalidDuration(t *testing.T) {
	g := NewWithT(t)

	_, _, err := extractTimeout([]string{"cmd", "--timeout", "invalid"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid timeout"))
}

func TestExtractTimeout_InvalidDurationEquals(t *testing.T) {
	g := NewWithT(t)

	_, _, err := extractTimeout([]string{"cmd", "--timeout=bad"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid timeout"))
}

// --- extractHelpFlag tests ---

func TestExtractHelpFlag_NotFound(t *testing.T) {
	g := NewWithT(t)

	found, remaining := extractHelpFlag([]string{"cmd", "arg1"})
	g.Expect(found).To(BeFalse())
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
}

func TestExtractHelpFlag_ShortFlag(t *testing.T) {
	g := NewWithT(t)

	found, remaining := extractHelpFlag([]string{"cmd", "-h", "arg1"})
	g.Expect(found).To(BeTrue())
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
}

func TestExtractHelpFlag_LongFlag(t *testing.T) {
	g := NewWithT(t)

	found, remaining := extractHelpFlag([]string{"cmd", "--help", "arg1"})
	g.Expect(found).To(BeTrue())
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
}

// --- Additional RunWithEnv tests ---

func TestRunWithEnv_ListCommand(t *testing.T) {
	g := NewWithT(t)

	cmd := &simpleRunCmd{}
	output := captureStdout(t, func() {
		env := &executeEnv{args: []string{"cmd", "__list"}}
		err := RunWithEnv(env, RunOptions{AllowDefault: true}, cmd)
		g.Expect(err).NotTo(HaveOccurred())
	})

	g.Expect(output).To(ContainSubstring("simple-run-cmd"))
}

func TestRunWithEnv_NoCommands(t *testing.T) {
	g := NewWithT(t)

	env := &executeEnv{args: []string{"cmd"}}
	err := RunWithEnv(env, RunOptions{}, []any{}...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env.Output()).To(ContainSubstring("No commands found"))
}

func TestRunWithEnv_UnknownCommand(t *testing.T) {
	g := NewWithT(t)

	cmd := &simpleRunCmd{}
	env := &executeEnv{args: []string{"cmd", "unknown-cmd"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: false}, cmd)
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("Unknown command"))
}

func TestRunWithEnv_CompletionFlag(t *testing.T) {
	g := NewWithT(t)

	cmd := &simpleRunCmd{}
	output := captureStdout(t, func() {
		env := &executeEnv{args: []string{"cmd", "--completion", "bash"}}
		err := RunWithEnv(env, RunOptions{AllowDefault: true}, cmd)
		g.Expect(err).NotTo(HaveOccurred())
	})

	// Should output a bash completion script (contains completion function definition)
	g.Expect(output).To(ContainSubstring("_completion"))
	g.Expect(output).To(ContainSubstring("complete"))
}

func TestRunWithEnv_TimeoutError(t *testing.T) {
	g := NewWithT(t)

	cmd := &simpleRunCmd{}
	env := &executeEnv{args: []string{"cmd", "--timeout", "invalid"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, cmd)
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("invalid"))
}

type simpleRunCmd struct{}

func (s *simpleRunCmd) Run() {}

// --- Helpers for other test files ---

// parseCommand is a helper used by completion_test.go
func parseCommand(f any) (*commandNode, error) {
	return parseStruct(f)
}
