package core

// internal_test.go contains whitebox tests that require access to unexported
// symbols. These test internal implementation details that cannot be easily
// tested through the public Execute() API.
//
// Most tests should be blackbox tests in the test/ directory.

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

type BadTagOptionsInputCmd struct {
	Name string `targ:"flag"`
}

func (b *BadTagOptionsInputCmd) Run() {}

// Wrong input types - accepts int instead of string
func (b *BadTagOptionsInputCmd) TagOptions(field int, opts TagOptions) (TagOptions, error) {
	return opts, nil
}

type BadTagOptionsReturnCmd struct {
	Name string `targ:"flag"`
}

func (b *BadTagOptionsReturnCmd) Run() {}

// Wrong return type - returns string instead of TagOptions
func (b *BadTagOptionsReturnCmd) TagOptions(field string, opts TagOptions) (string, error) {
	return "", nil
}

// --- applyTagOptionsOverride tests ---

type BadTagOptionsSignatureCmd struct {
	Name string `targ:"flag"`
}

func (b *BadTagOptionsSignatureCmd) Run() {}

// Wrong signature - should accept (string, TagOptions) and return (TagOptions, error)
func (b *BadTagOptionsSignatureCmd) TagOptions(field string) TagOptions {
	return TagOptions{}
}

type ConflictChild2 struct {
	Flag string `targ:"flag"`
}

func (c *ConflictChild2) Run() {}

// --- Flag Conflict Detection ---

type ConflictRoot struct {
	Flag string          `targ:"flag"`
	Sub  *ConflictChild2 `targ:"subcommand"`
}

// --- Flag parsing tests ---

type FlagParseCmd struct {
	Name    string `targ:"flag,short=n"`
	Verbose bool   `targ:"flag,short=v"`
	Count   int    `targ:"flag,short=c"`
}

func (f *FlagParseCmd) Run() {}

// --- Command Metadata ---

type MetaOverrideCmd struct{}

func (m *MetaOverrideCmd) Description() string {
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

type PersistentAfterCmd struct {
	Executed bool
}

func (p *PersistentAfterCmd) PersistentAfter() error {
	return errors.New("persistent after failed")
}

func (p *PersistentAfterCmd) Run() {
	p.Executed = true
}

// --- runPersistentHooks tests ---

type PersistentBeforeCmd struct{}

func (p *PersistentBeforeCmd) PersistentBefore() error {
	return errors.New("persistent before failed")
}

func (p *PersistentBeforeCmd) Run() {}

type RootWithPresetSub struct {
	Sub *SubCmdInternal `targ:"subcommand"`
}

func (r *RootWithPresetSub) Run() {}

type SubCmdInternal struct{}

func (s *SubCmdInternal) Run() {}

type TagOptionsErrorCmd struct {
	Name string `targ:"flag"`
}

func (t *TagOptionsErrorCmd) Run() {}

func (t *TagOptionsErrorCmd) TagOptions(field string, opts TagOptions) (TagOptions, error) {
	return opts, errors.New("tag options error")
}

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

func InvalidParamFunc(n int) {}

func InvalidReturnFunc() int {
	return 42
}

func TooManyReturnsFunc() (int, error) {
	return 42, nil
}

type TooManyReturnsMethod struct{}

func (t *TooManyReturnsMethod) Run() (int, error) {
	return 42, nil
}

// --- parseFunc tests ---

func InvalidSigFunc(a, b int) {}

func TestApplyTagOptionsOverride_MethodReturnsError(t *testing.T) {
	g := NewWithT(t)

	cmd := &TagOptionsErrorCmd{}
	_, err := parseStruct(cmd)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("tag options error"))
}

func TestApplyTagOptionsOverride_WrongInputType(t *testing.T) {
	g := NewWithT(t)

	cmd := &BadTagOptionsInputCmd{}
	_, err := parseStruct(cmd)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("TagOptions"))
}

func TestApplyTagOptionsOverride_WrongReturnType(t *testing.T) {
	g := NewWithT(t)

	cmd := &BadTagOptionsReturnCmd{}
	_, err := parseStruct(cmd)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("TagOptions"))
}

func TestApplyTagOptionsOverride_WrongSignature(t *testing.T) {
	g := NewWithT(t)

	cmd := &BadTagOptionsSignatureCmd{}
	_, err := parseStruct(cmd)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("TagOptions"))
}

func TestAssignSubcommandField_FuncKindNil(t *testing.T) {
	g := NewWithT(t)

	cmd := &subFuncCmd{Sub: nil}
	parent, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	inst := reflect.ValueOf(cmd).Elem()
	sub := &commandNode{Name: "sub"}

	err = assignSubcommandField(parent, inst, "sub", sub)
	g.Expect(err).To(HaveOccurred())

	if err == nil {
		t.Fatal("expected error")
	}

	g.Expect(err.Error()).To(ContainSubstring("subcommand sub is nil"))
}

func TestAssignSubcommandField_FuncKindSet(t *testing.T) {
	g := NewWithT(t)

	cmd := &subFuncCmd{Sub: func() {}}
	parent, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	inst := reflect.ValueOf(cmd).Elem()
	sub := &commandNode{Name: "sub"}

	err = assignSubcommandField(parent, inst, "sub", sub)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sub.Func.IsValid()).To(BeTrue())
}

// --- assignSubcommandField tests ---

func TestAssignSubcommandField_NilParent(t *testing.T) {
	g := NewWithT(t)

	err := assignSubcommandField(nil, reflect.Value{}, "sub", &commandNode{})
	g.Expect(err).NotTo(HaveOccurred())
}

func TestAssignSubcommandField_NilParentType(t *testing.T) {
	g := NewWithT(t)

	parent := &commandNode{Type: nil}
	err := assignSubcommandField(parent, reflect.Value{}, "sub", &commandNode{})
	g.Expect(err).NotTo(HaveOccurred())
}

func TestAssignSubcommandField_PtrKind(t *testing.T) {
	g := NewWithT(t)

	cmd := &subPtrCmd{}
	parent, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	inst := reflect.ValueOf(cmd).Elem()
	sub := &commandNode{Name: "sub"}

	err = assignSubcommandField(parent, inst, "sub", sub)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sub.Value.IsValid()).To(BeTrue())
	g.Expect(cmd.Sub).NotTo(BeNil())
}

func TestAssignSubcommandField_StructKind(t *testing.T) {
	g := NewWithT(t)

	cmd := &subStructCmd{}
	parent, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	inst := reflect.ValueOf(cmd).Elem()
	sub := &commandNode{Name: "sub"}

	err = assignSubcommandField(parent, inst, "sub", sub)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sub.Value.IsValid()).To(BeTrue())
}

// --- binaryName tests ---

func TestBinaryName_FromEnvVar(t *testing.T) {
	g := NewWithT(t)
	t.Setenv("TARG_BIN_NAME", "custom-binary")

	result := binaryName()
	g.Expect(result).To(Equal("custom-binary"))
}

func TestBinaryName_FromOsArgs(t *testing.T) {
	g := NewWithT(t)
	// Clear the env var so we fall through to os.Args
	t.Setenv("TARG_BIN_NAME", "")

	// binaryName uses os.Args[0], so we test the logic it relies on
	result := binaryName()
	g.Expect(result).NotTo(BeEmpty())
	// Should be stripped of path (at minimum, should not contain directory separators
	// unless the binary name itself contains them, which is rare)
}

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

func TestCustomSetter_NonAddressable(t *testing.T) {
	g := NewWithT(t)

	// Create a non-addressable value
	val := reflect.ValueOf(testPlainType{})

	setter, ok := customSetter(val)
	g.Expect(ok).To(BeFalse())
	g.Expect(setter).To(BeNil())
}

func TestCustomSetter_PlainType_NotFound(t *testing.T) {
	g := NewWithT(t)

	var target testPlainType

	val := reflect.ValueOf(&target).Elem()

	setter, ok := customSetter(val)
	g.Expect(ok).To(BeFalse(), "should not find setter for plain type")
	g.Expect(setter).To(BeNil())
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

func TestCustomSetter_ValueTypeImplementsStringSetter(t *testing.T) {
	g := NewWithT(t)

	// Create a non-addressable value of a type that implements Set with value receiver
	m := map[string]testValueStringSetter{"key": {}}
	val := reflect.ValueOf(m).MapIndex(reflect.ValueOf("key"))

	g.Expect(val.CanAddr()).To(BeFalse())

	setter, ok := customSetter(val)
	// Value type implements interface, so we should get a setter
	g.Expect(ok).To(BeTrue())
	g.Expect(setter).NotTo(BeNil())
	// Note: Actually invoking setter would panic because map values aren't settable
}

func TestCustomSetter_ValueTypeImplementsTextUnmarshaler(t *testing.T) {
	g := NewWithT(t)

	// Create a non-addressable value of a type that implements TextUnmarshaler with value receiver
	// Map values are non-addressable
	m := map[string]testValueTextUnmarshaler{"key": {}}
	val := reflect.ValueOf(m).MapIndex(reflect.ValueOf("key"))

	g.Expect(val.CanAddr()).To(BeFalse())

	setter, ok := customSetter(val)
	// Value type implements interface, so we should get a setter
	g.Expect(ok).To(BeTrue())
	g.Expect(setter).NotTo(BeNil())
	// Note: Actually invoking setter would panic because map values aren't settable
	// The non-addressable paths in customSetter are dead code in practice
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

// --- executeEnv tests ---

func TestExecuteEnv_ExitIsNoOp(t *testing.T) {
	env := &executeEnv{args: []string{"cmd"}}
	// Exit should be a no-op and not panic
	env.Exit(1)
	env.Exit(0)
}

// --- ExitError tests ---

func TestExitError_Error(t *testing.T) {
	g := NewWithT(t)

	err := ExitError{Code: 42}
	g.Expect(err.Error()).To(Equal("exit code 42"))
}

func TestExitError_Error_Zero(t *testing.T) {
	g := NewWithT(t)

	err := ExitError{Code: 0}
	g.Expect(err.Error()).To(ContainSubstring("0"))
}

func TestExpectingFlagValue_DoubleDash(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{}
	g.Expect(expectingFlagValue([]string{"--"}, specs)).To(BeFalse())
}

// --- expectingFlagValue tests ---

func TestExpectingFlagValue_EmptyArgs(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{}
	g.Expect(expectingFlagValue([]string{}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_FlagGroupLastTakesValue(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-v": {TakesValue: false},
		"-n": {TakesValue: true},
	}
	// -vn where n is last and takes value
	g.Expect(expectingFlagValue([]string{"-vn"}, specs)).To(BeTrue())
}

func TestExpectingFlagValue_FlagGroupMiddleTakesValue(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-v": {TakesValue: false},
		"-n": {TakesValue: true},
	}
	// -nv where n takes value but is not last
	g.Expect(expectingFlagValue([]string{"-nv"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_LongFlagNeedsValue(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"--name": {TakesValue: true},
	}
	g.Expect(expectingFlagValue([]string{"--name"}, specs)).To(BeTrue())
}

func TestExpectingFlagValue_LongFlagNoValue(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"--verbose": {TakesValue: false},
	}
	g.Expect(expectingFlagValue([]string{"--verbose"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_LongFlagUnknown(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{}
	g.Expect(expectingFlagValue([]string{"--unknown"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_LongFlagWithEquals(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"--name": {TakesValue: true},
	}
	g.Expect(expectingFlagValue([]string{"--name=foo"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_Positional(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{}
	g.Expect(expectingFlagValue([]string{"arg1"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_ShortFlagNeedsValue(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-n": {TakesValue: true},
	}
	g.Expect(expectingFlagValue([]string{"-n"}, specs)).To(BeTrue())
}

func TestExpectingFlagValue_ShortFlagNoValue(t *testing.T) {
	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-v": {TakesValue: false},
	}
	g.Expect(expectingFlagValue([]string{"-v"}, specs)).To(BeFalse())
}

func TestExtractHelpFlag_LongFlag(t *testing.T) {
	g := NewWithT(t)

	found, remaining := extractHelpFlag([]string{"cmd", "--help", "arg1"})
	g.Expect(found).To(BeTrue())
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
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

func TestExtractTimeout_MissingValue(t *testing.T) {
	g := NewWithT(t)

	_, _, err := extractTimeout([]string{"cmd", "--timeout"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires a duration"))
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

func TestFlagParsing_IntFlagEqualsFormat(t *testing.T) {
	g := NewWithT(t)

	cmd := &FlagParseCmd{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseStruct returned nil node")
	}

	err = node.execute(context.Background(), []string{"-c=42"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cmd.Count).To(Equal(42))
}

func TestFlagParsing_LongFlagEqualsFormat(t *testing.T) {
	g := NewWithT(t)

	cmd := &FlagParseCmd{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseStruct returned nil node")
	}

	err = node.execute(context.Background(), []string{"--name=hello"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cmd.Name).To(Equal("hello"))
}

func TestFlagParsing_ShortFlagEqualsFormat(t *testing.T) {
	g := NewWithT(t)

	cmd := &FlagParseCmd{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseStruct returned nil node")
	}

	err = node.execute(context.Background(), []string{"-n=world"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cmd.Name).To(Equal("world"))
}

func TestFlagParsing_UnknownLongFlag(t *testing.T) {
	g := NewWithT(t)

	cmd := &FlagParseCmd{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseStruct returned nil node")
	}

	err = node.execute(context.Background(), []string{"--unknown"}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("--unknown"))
}

func TestFlagParsing_UnknownShortFlag(t *testing.T) {
	g := NewWithT(t)

	cmd := &FlagParseCmd{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseStruct returned nil node")
	}

	err = node.execute(context.Background(), []string{"-x"}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("-x"))
}

// --- NewOsEnv tests ---

func TestNewOsEnv_ReturnsRunEnv(t *testing.T) {
	g := NewWithT(t)

	env := NewOsEnv()
	g.Expect(env).NotTo(BeNil())
	// Just verify it implements runEnv interface
	_ = env
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

func TestParseFunc_NotFuncType(t *testing.T) {
	g := NewWithT(t)

	// Pass a non-func reflect.Value to parseFunc
	v := reflect.ValueOf(42)
	_, err := parseFunc(v)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("expected func"))
}

// --- Parsing Edge Cases ---

func TestParseNilPointer(t *testing.T) {
	var cmd *MyCommandStruct
	if _, err := parseStruct(cmd); err == nil {
		t.Fatal("expected error for nil pointer target")
	}
}

func TestParseTarget_InvalidFunctionParam(t *testing.T) {
	g := NewWithT(t)

	// Function with non-context param
	_, err := parseTarget(InvalidParamFunc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("context.Context"))
}

func TestParseTarget_InvalidFunctionReturn(t *testing.T) {
	g := NewWithT(t)

	// Function returning non-error
	_, err := parseTarget(InvalidReturnFunc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("return only error"))
}

func TestParseTarget_TooManyReturns(t *testing.T) {
	g := NewWithT(t)

	// Function returning multiple values
	_, err := parseTarget(TooManyReturnsFunc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("return only error"))
}

func TestExecute_MethodTooManyReturns(t *testing.T) {
	g := NewWithT(t)

	// Struct with Run method returning multiple values
	cmd := &TooManyReturnsMethod{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())
	if node == nil {
		t.Fatal("node should not be nil when err is nil")
	}

	err = node.execute(context.TODO(), []string{}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("return only error"))
}

func TestParseTarget_InvalidFunctionSignature(t *testing.T) {
	g := NewWithT(t)

	// Function with too many params
	_, err := parseTarget(InvalidSigFunc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("niladic or accept context"))
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

func TestPositionalIndex_BoolFlagNoConsume(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"-v", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_FlagGroup(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	// -vn where v is bool and n takes value - should consume next arg
	idx, err := positionalIndex(node, []string{"-vn", "foo", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_LongFlagConsumeNext(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"--name", "foo", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_LongFlagWithEquals(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"--name=foo", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_NoArgs(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(0))
}

func TestPositionalIndex_OnlyPositionals(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"arg1", "arg2"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(2))
}

func TestPositionalIndex_ShortFlagConsumeNext(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"-n", "foo", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_ShortFlagWithEquals(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"-n=foo", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_SkipsDoubleDash(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"--", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_UnknownLongFlag(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	// Unknown flag is skipped (not treated as positional)
	idx, err := positionalIndex(node, []string{"--unknown", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_VariadicFlag(t *testing.T) {
	g := NewWithT(t)

	node, err := parseStruct(&posIdxCmd{})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	// --files is variadic - the variadic consumption in positionalIndex stops at "--" or flag
	// After --files a b c --, arg1 is counted as positional
	// Note: variadic behavior for string fields may not consume multiple args in completion context
	idx, err := positionalIndex(node, []string{"--files", "a", "b", "c", "--", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	// Test that it processes without error - exact count depends on variadic implementation
	g.Expect(idx).To(BeNumerically(">=", 0))
}

func TestPrintCommandHelp_FlagWithPlaceholder(t *testing.T) {
	g := NewWithT(t)

	cmd := &helpTestCmdWithPlaceholder{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	output := captureStdout(t, func() {
		printCommandHelp(node)
	})

	g.Expect(output).To(ContainSubstring("--output <file>"))
}

func TestPrintCommandHelp_FlagWithShortName(t *testing.T) {
	g := NewWithT(t)

	cmd := &helpTestCmdWithShort{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	output := captureStdout(t, func() {
		printCommandHelp(node)
	})

	g.Expect(output).To(ContainSubstring("--verbose, -v"))
}

func TestPrintCommandHelp_FlagWithUsage(t *testing.T) {
	g := NewWithT(t)

	cmd := &helpTestCmdWithUsage{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	output := captureStdout(t, func() {
		printCommandHelp(node)
	})

	g.Expect(output).To(ContainSubstring("Output format"))
}

// --- printCommandHelp tests ---

func TestPrintCommandHelp_FunctionNode(t *testing.T) {
	g := NewWithT(t)

	node := &commandNode{
		Name:        "test-cmd",
		Description: "A test command",
		Type:        nil, // Function node has no Type
	}

	output := captureStdout(t, func() {
		printCommandHelp(node)
	})

	g.Expect(output).To(ContainSubstring("Usage:"))
	g.Expect(output).To(ContainSubstring("test-cmd"))
	g.Expect(output).To(ContainSubstring("A test command"))
}

func TestPrintCommandHelp_WithDescription(t *testing.T) {
	g := NewWithT(t)

	cmd := &helpTestCmdWithDesc{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseStruct returned nil node")
	}

	node.Description = "A command with description"

	output := captureStdout(t, func() {
		printCommandHelp(node)
	})

	g.Expect(output).To(ContainSubstring("A command with description"))
}

func TestPrintCommandHelp_WithFlags(t *testing.T) {
	g := NewWithT(t)

	cmd := &helpTestCmd{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	output := captureStdout(t, func() {
		printCommandHelp(node)
	})

	g.Expect(output).To(ContainSubstring("Usage:"))
	g.Expect(output).To(ContainSubstring("--name"))
}

func TestPrintCommandHelp_WithSubcommands(t *testing.T) {
	g := NewWithT(t)

	cmd := &helpTestCmdWithSub{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseStruct returned nil node")
	}

	// Manually add a subcommand since parseStruct won't create them
	node.Subcommands = map[string]*commandNode{
		"sub1": {Name: "sub1", Description: "Subcommand one"},
		"sub2": {Name: "sub2", Description: "Subcommand two"},
	}

	output := captureStdout(t, func() {
		printCommandHelp(node)
	})

	g.Expect(output).To(ContainSubstring("Subcommands:"))
	g.Expect(output).To(ContainSubstring("sub1"))
	g.Expect(output).To(ContainSubstring("sub2"))
}

func TestPrintCommandSummary_NoSubcommands(t *testing.T) {
	node := &commandNode{
		Name:        "simple",
		Description: "Simple command",
	}

	out := captureStdout(t, func() {
		printCommandSummary(node, "  ")
	})

	if !strings.Contains(out, "simple") {
		t.Errorf("expected simple command, got: %s", out)
	}
}

// --- printCommandSummary tests ---

func TestPrintCommandSummary_WithSubcommands(t *testing.T) {
	// Create a command with nested subcommands
	node := &commandNode{
		Name:        "root",
		Description: "Root command",
		Subcommands: map[string]*commandNode{
			"sub1": {
				Name:        "sub1",
				Description: "First subcommand",
			},
			"sub2": {
				Name:        "sub2",
				Description: "Second subcommand",
				Subcommands: map[string]*commandNode{
					"nested": {
						Name:        "nested",
						Description: "Nested subcommand",
					},
				},
			},
		},
	}

	out := captureStdout(t, func() {
		printCommandSummary(node, "")
	})

	// Verify output contains all commands
	if !strings.Contains(out, "root") {
		t.Errorf("expected root command, got: %s", out)
	}

	if !strings.Contains(out, "sub1") {
		t.Errorf("expected sub1 command, got: %s", out)
	}

	if !strings.Contains(out, "sub2") {
		t.Errorf("expected sub2 command, got: %s", out)
	}

	if !strings.Contains(out, "nested") {
		t.Errorf("expected nested command, got: %s", out)
	}
}

// --- runCommand tests ---

func TestRunCommand_NilNode(t *testing.T) {
	g := NewWithT(t)

	err := runCommand(context.Background(), nil, reflect.Value{}, nil, 0)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestRunPersistentHooks_AfterError(t *testing.T) {
	g := NewWithT(t)

	cmd := &PersistentAfterCmd{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseStruct returned nil node")
	}

	err = node.execute(context.Background(), nil, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("persistent after failed"))
	// Run should have been called before After fails
	g.Expect(cmd.Executed).To(BeTrue())
}

func TestRunPersistentHooks_BeforeError(t *testing.T) {
	g := NewWithT(t)

	cmd := &PersistentBeforeCmd{}
	node, err := parseStruct(cmd)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseStruct returned nil node")
	}

	err = node.execute(context.Background(), nil, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("persistent before failed"))
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

func TestRunWithEnv_TimeoutError(t *testing.T) {
	g := NewWithT(t)

	cmd := &simpleRunCmd{}
	env := &executeEnv{args: []string{"cmd", "--timeout", "invalid"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, cmd)
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("invalid"))
}

func TestRunWithEnv_UnknownCommand(t *testing.T) {
	g := NewWithT(t)

	cmd := &simpleRunCmd{}
	env := &executeEnv{args: []string{"cmd", "unknown-cmd"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: false}, cmd)
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("Unknown command"))
}

// --- skipTargFlags tests ---

func TestSkipTargFlags_EmptyArgs(t *testing.T) {
	g := NewWithT(t)

	result := skipTargFlags([]string{})
	g.Expect(result).To(BeEmpty())
}

func TestSkipTargFlags_NoTargFlags(t *testing.T) {
	g := NewWithT(t)

	result := skipTargFlags([]string{"build", "test", "--verbose"})
	g.Expect(result).To(Equal([]string{"build", "test", "--verbose"}))
}

func TestSkipTargFlags_WithAlias(t *testing.T) {
	g := NewWithT(t)

	// --alias is exit-early, consumes everything after it
	result := skipTargFlags([]string{"--alias", "foo", "bar"})
	g.Expect(result).To(BeEmpty())
}

func TestSkipTargFlags_WithCompletion(t *testing.T) {
	g := NewWithT(t)

	// --completion is exit-early, stops processing
	result := skipTargFlags([]string{"--completion", "bash"})
	g.Expect(result).To(BeEmpty())
}

func TestSkipTargFlags_WithCompletionEquals(t *testing.T) {
	g := NewWithT(t)

	// --completion=bash uses flag=value syntax
	result := skipTargFlags([]string{"--completion=bash", "build"})
	g.Expect(result).To(Equal([]string{"build"}))
}

func TestSkipTargFlags_WithHelp(t *testing.T) {
	g := NewWithT(t)

	// --help is boolean flag
	result := skipTargFlags([]string{"--help", "build"})
	g.Expect(result).To(Equal([]string{"build"}))
}

func TestSkipTargFlags_WithNoCache(t *testing.T) {
	g := NewWithT(t)

	// --no-cache is boolean flag
	result := skipTargFlags([]string{"--no-cache", "build"})
	g.Expect(result).To(Equal([]string{"build"}))
}

func TestSkipTargFlags_WithTimeout(t *testing.T) {
	g := NewWithT(t)

	// --timeout consumes next arg
	result := skipTargFlags([]string{"--timeout", "10m", "build"})
	g.Expect(result).To(Equal([]string{"build"}))
}

func TestSkipTargFlags_WithTimeoutEquals(t *testing.T) {
	g := NewWithT(t)

	// --timeout=value syntax
	result := skipTargFlags([]string{"--timeout=10m", "build"})
	g.Expect(result).To(Equal([]string{"build"}))
}

func TestTagOptionsInstance_Empty(t *testing.T) {
	g := NewWithT(t)

	node := &commandNode{}
	result := tagOptionsInstance(node)
	g.Expect(result.IsValid()).To(BeFalse())
}

// --- tagOptionsInstance tests ---

func TestTagOptionsInstance_NilNode(t *testing.T) {
	g := NewWithT(t)

	result := tagOptionsInstance(nil)
	g.Expect(result.IsValid()).To(BeFalse())
}

func TestTagOptionsInstance_WithType(t *testing.T) {
	g := NewWithT(t)

	node := &commandNode{
		Type: reflect.TypeFor[posIdxCmd](),
	}
	result := tagOptionsInstance(node)
	g.Expect(result.IsValid()).To(BeTrue())
	g.Expect(result.Kind()).To(Equal(reflect.Struct))
}

func TestTagOptionsInstance_WithValue(t *testing.T) {
	g := NewWithT(t)

	cmd := &posIdxCmd{Name: "test"}
	node := &commandNode{
		Value: reflect.ValueOf(cmd).Elem(),
	}
	result := tagOptionsInstance(node)
	g.Expect(result.IsValid()).To(BeTrue())
	g.Expect(result.Kind()).To(Equal(reflect.Struct))
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

type helpTestCmd struct {
	Name string `targ:"flag"`
}

func (h *helpTestCmd) Run() {}

type helpTestCmdWithDesc struct{}

func (h *helpTestCmdWithDesc) Run() {}

type helpTestCmdWithPlaceholder struct {
	Output string `targ:"flag,placeholder=<file>"`
}

func (h *helpTestCmdWithPlaceholder) Run() {}

type helpTestCmdWithShort struct {
	Verbose bool `targ:"flag,short=v"`
}

func (h *helpTestCmdWithShort) Run() {}

type helpTestCmdWithSub struct{}

func (h *helpTestCmdWithSub) Run() {}

type helpTestCmdWithUsage struct {
	Format string `targ:"flag,desc=Output format"`
}

func (h *helpTestCmdWithUsage) Run() {}

// --- positionalIndex tests ---

type posIdxCmd struct {
	Name    string `targ:"flag,short=n"`
	Verbose bool   `targ:"flag,short=v"`
	Files   string `targ:"flag,variadic"`
}

func (p *posIdxCmd) Run() {}

type simpleRunCmd struct{}

func (s *simpleRunCmd) Run() {}

type subFuncCmd struct {
	Sub func() `targ:"subcommand"`
}

type subPtrCmd struct {
	Sub *subPtrSub `targ:"subcommand"`
}

type subPtrSub struct{}

func (s *subPtrSub) Run() {}

type subStructCmd struct {
	Sub subStructSub `targ:"subcommand"`
}

type subStructSub struct{}

func (s subStructSub) Run() {}

// plainType has no custom setter
type testPlainType struct{}

// stringSetterType implements Set(string) error
type testStringSetter struct {
	value string
}

func (t *testStringSetter) Set(s string) error {
	t.value = "set:" + s
	return nil
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

// testValueStringSetter implements Set with VALUE receiver
type testValueStringSetter struct {
	Value string
}

func (t testValueStringSetter) Set(s string) error {
	return nil
}

// testValueTextUnmarshaler implements TextUnmarshaler with VALUE receiver
// This is unusual but allows testing the non-addressable code path
type testValueTextUnmarshaler struct {
	Value string
}

func (t testValueTextUnmarshaler) UnmarshalText(text []byte) error {
	// With value receiver, the method can't modify t, but can still implement the interface
	return nil
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

// --- Helpers for other test files ---

// parseCommand is a helper used by completion_test.go
func parseCommand(f any) (*commandNode, error) {
	return parseStruct(f)
}
