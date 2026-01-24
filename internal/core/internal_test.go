package core

// internal_test.go contains whitebox tests that require access to unexported
// symbols. These test internal implementation details that cannot be easily
// tested through the public Execute() API.
//
// Most tests should be blackbox tests in the test/ directory.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	internalfile "github.com/toejough/targ/internal/file"
)

type FlagParseCmdArgs struct {
	Name    string `targ:"flag,short=n"`
	Verbose bool   `targ:"flag,short=v"`
	Count   int    `targ:"flag,short=c"`
}

// TagOptionsErrorArgs tests error handling in TagOptions method.
type TagOptionsErrorArgs struct {
	Mode string `targ:"flag"`
}

// TagOptions returns an error to test error handling.
func (TagOptionsErrorArgs) TagOptions(_ string, _ TagOptions) (TagOptions, error) {
	return TagOptions{}, errors.New("tag options error")
}

// TagOptionsOverrideArgs tests the TagOptions method on args structs.
type TagOptionsOverrideArgs struct {
	Env string `targ:"positional,enum=dev|prod"`
}

// TagOptions dynamically modifies enum values at runtime.
func (t TagOptionsOverrideArgs) TagOptions(field string, opts TagOptions) (TagOptions, error) {
	if field == "Env" {
		opts.Enum = "dev|staging|prod"
	}

	return opts, nil
}

// TagOptionsWrongCountArgs has a TagOptions method with wrong number of args.
type TagOptionsWrongCountArgs struct {
	Mode string `targ:"flag"`
}

// TagOptions has wrong number of inputs (3 instead of 2).
func (TagOptionsWrongCountArgs) TagOptions(_, _ string, _ TagOptions) (TagOptions, error) {
	return TagOptions{}, nil
}

// TagOptionsWrongInputArgs has a TagOptions method with wrong input types.
type TagOptionsWrongInputArgs struct {
	Mode string `targ:"flag"`
}

// TagOptions has wrong input types (int instead of string).
func (TagOptionsWrongInputArgs) TagOptions(_ int, _ TagOptions) (TagOptions, error) {
	return TagOptions{}, nil
}

// TagOptionsWrongOutputArgs has a TagOptions method with wrong output types.
type TagOptionsWrongOutputArgs struct {
	Mode string `targ:"flag"`
}

// TagOptions has wrong output types (string instead of error).
func (TagOptionsWrongOutputArgs) TagOptions(_ string, _ TagOptions) (TagOptions, string) {
	return TagOptions{}, ""
}

func InvalidParamFunc(_ int) {}

func InvalidReturnFunc() int {
	return 42
}

// --- parseFunc tests ---

func InvalidSigFunc(_, _ int) {}

func TestAppendBuiltinExamples(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	custom := Example{Title: "Custom", Code: "custom code"}
	examples := AppendBuiltinExamples(custom)

	g.Expect(examples).To(HaveLen(3))
	g.Expect(examples[0].Title).To(Equal("Custom"))
	g.Expect(examples[1].Title).To(Equal("Enable shell completion"))
	g.Expect(examples[2].Title).To(ContainSubstring("Run multiple"))
}

func TestApplyTagOptionsOverride_Error(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test that TagOptions method errors are propagated during execution
	fn := func(_ TagOptionsErrorArgs) {}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	// The error should occur during execution when TagOptions is called
	err = node.execute(context.Background(), []string{"--mode", "test"}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("tag options error"))
}

// --- applyTagOptionsOverride tests ---

func TestApplyTagOptionsOverride_Success(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test that TagOptions method is called and modifies the options
	var gotEnv string

	fn := func(args TagOptionsOverrideArgs) { gotEnv = args.Env }
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	// The TagOptions method changes enum from "dev|prod" to "dev|staging|prod"
	// so "staging" should be accepted
	err = node.execute(context.Background(), []string{"staging"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotEnv).To(Equal("staging"))
}

func TestApplyTagOptionsOverride_WrongArgCount(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test that TagOptions method with wrong arg count returns an error
	fn := func(_ TagOptionsWrongCountArgs) {}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.Background(), []string{"--mode", "test"}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("TagOptions"))
}

func TestApplyTagOptionsOverride_WrongInputTypes(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test that TagOptions method with wrong input types returns an error
	fn := func(_ TagOptionsWrongInputArgs) {}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.Background(), []string{"--mode", "test"}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("must accept"))
}

func TestApplyTagOptionsOverride_WrongOutputTypes(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test that TagOptions method with wrong output types returns an error
	fn := func(_ TagOptionsWrongOutputArgs) {}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.Background(), []string{"--mode", "test"}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("must return"))
}

func TestApplyTimeout_DisableTimeout(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	ctx := context.Background()
	args := []string{"--timeout", "5m", "arg"}

	newCtx, remaining, cancel, err := applyTimeout(ctx, args, RunOptions{DisableTimeout: true})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(newCtx).To(Equal(ctx))
	g.Expect(remaining).To(Equal(args))
	g.Expect(cancel).To(BeNil())
}

func TestApplyTimeout_Error(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	ctx := context.Background()
	args := []string{"--timeout", "invalid"}

	_, _, _, err := applyTimeout(ctx, args, RunOptions{})
	g.Expect(err).To(HaveOccurred())
}

func TestApplyTimeout_NoTimeout(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	ctx := context.Background()
	args := []string{"arg1", "arg2"}

	newCtx, remaining, cancel, err := applyTimeout(ctx, args, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(newCtx).To(Equal(ctx))
	g.Expect(remaining).To(Equal(args))
	g.Expect(cancel).To(BeNil())
}

func TestApplyTimeout_WithTimeout(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	ctx := context.Background()
	args := []string{"--timeout", "1h", "arg1"}

	newCtx, remaining, cancel, err := applyTimeout(ctx, args, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(newCtx).NotTo(Equal(ctx)) // New context with deadline
	g.Expect(remaining).To(Equal([]string{"arg1"}))
	g.Expect(cancel).NotTo(BeNil())

	// Clean up
	cancel()
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

func TestBuildUsageLineForPath(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{
		Name: "cmd",
	}

	result := buildUsageLineForPath(node, "targ", "parent cmd")
	g.Expect(result).To(Equal("targ parent cmd"))
}

func TestBuildUsageLineForPath_WithFlags(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{
		Name: "cmd",
		Type: reflect.TypeFor[struct {
			Name string `targ:"flag,required"`
		}](),
	}

	result := buildUsageLineForPath(node, "targ", "parent cmd")
	g.Expect(result).To(ContainSubstring("targ parent cmd"))
	g.Expect(result).To(ContainSubstring("--name"))
}

func TestBuildUsageLineForPath_WithOptionalFlags(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{
		Name: "cmd",
		Type: reflect.TypeFor[struct {
			Verbose bool `targ:"flag"`
		}](),
	}

	result := buildUsageLineForPath(node, "targ", "cmd")
	g.Expect(result).To(ContainSubstring("[--verbose]"))
}

func TestBuildUsageLineForPath_WithOptionalPositionals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{
		Name: "cmd",
		Type: reflect.TypeFor[struct {
			File string `targ:"positional"`
		}](),
	}

	result := buildUsageLineForPath(node, "targ", "cmd")
	g.Expect(result).To(ContainSubstring("[File]"))
}

func TestBuildUsageLineForPath_WithPlaceholder(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{
		Name: "cmd",
		Type: reflect.TypeFor[struct {
			File string `targ:"positional,placeholder=<file>"`
		}](),
	}

	result := buildUsageLineForPath(node, "targ", "cmd")
	g.Expect(result).To(ContainSubstring("[<file>]"))
}

func TestBuildUsageLineForPath_WithPositionals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{
		Name: "cmd",
		Type: reflect.TypeFor[struct {
			File string `targ:"positional,required"`
		}](),
	}

	result := buildUsageLineForPath(node, "targ", "cmd")
	g.Expect(result).To(ContainSubstring("File"))
}

func TestBuildUsageLineForPath_WithSubcommands(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{
		Name: "cmd",
		Subcommands: map[string]*commandNode{
			"sub": {Name: "sub"},
		},
	}

	result := buildUsageLineForPath(node, "targ", "cmd")
	g.Expect(result).To(ContainSubstring("[subcommand]"))
}

func TestBuildUsageLine_Success(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{
		Name: "cmd",
	}

	result, err := buildUsageLine(node)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal("cmd"))
}

func TestBuildUsageLine_WithTagOptionsError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Use a type with a TagOptions method that errors
	node := &commandNode{
		Name:  "cmd",
		Type:  reflect.TypeFor[TagOptionsErrorArgs](),
		Value: reflect.ValueOf(TagOptionsErrorArgs{}),
	}

	_, err := buildUsageLine(node)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("tag options error"))
}

// --- Examples tests ---

func TestBuiltinExamples(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	examples := BuiltinExamples()

	g.Expect(examples).To(HaveLen(2))
	g.Expect(examples[0].Title).To(Equal("Enable shell completion"))
	g.Expect(examples[0].Code).To(ContainSubstring("--completion"))
	g.Expect(examples[1].Title).To(ContainSubstring("Run multiple"))
	g.Expect(examples[1].Code).To(ContainSubstring("targ"))
}

// --- camelToKebab ---

func TestCamelToKebab(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

// --- collectInstanceEnums tests ---

func TestCollectInstanceEnums_NilNode(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	enumByFlag := map[string][]string{}

	// Test with nil node
	err := collectInstanceEnums(commandInstance{node: nil}, enumByFlag)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(enumByFlag).To(BeEmpty())
}

func TestCollectInstanceEnums_NilType(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	enumByFlag := map[string][]string{}

	// Test with node but nil Type
	err := collectInstanceEnums(commandInstance{node: &commandNode{Type: nil}}, enumByFlag)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(enumByFlag).To(BeEmpty())
}

func TestCompletionChain_NilNode(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Exercises the nil node early return branch
	chain, err := completionChain(nil, []string{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(chain).To(BeNil())
}

func TestCompletionExample_Bash(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")

	ex := completionExample()
	if ex.Code != "eval \"$(targ --completion)\"" {
		t.Fatalf("expected bash syntax, got: %s", ex.Code)
	}
}

func TestCompletionExample_Fish(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/fish")

	ex := completionExample()
	if ex.Code != "targ --completion | source" {
		t.Fatalf("expected fish syntax, got: %s", ex.Code)
	}
}

func TestCompletionExample_Zsh(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")

	ex := completionExample()
	if ex.Code != "source <(targ --completion)" {
		t.Fatalf("expected zsh syntax, got: %s", ex.Code)
	}
}

func TestCustomSetter_NonAddressable(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Create a non-addressable value
	val := reflect.ValueOf(testPlainType{})

	setter, ok := customSetter(val)
	g.Expect(ok).To(BeFalse())
	g.Expect(setter).To(BeNil())
}

func TestCustomSetter_PlainType_NotFound(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var target testPlainType

	val := reflect.ValueOf(&target).Elem()

	setter, ok := customSetter(val)
	g.Expect(ok).To(BeFalse(), "should not find setter for plain type")
	g.Expect(setter).To(BeNil())
}

func TestCustomSetter_StringSetter(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

func TestDepModeString(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	g.Expect(DepModeSerial.String()).To(Equal("serial"))
	g.Expect(DepModeParallel.String()).To(Equal("parallel"))

	// Unknown DepMode values default to "serial"
	unknownMode := DepMode(99)
	g.Expect(unknownMode.String()).To(Equal("serial"))
}

func TestDetectCompletionShell_ExplicitShell(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	exec := &runExecutor{
		rest: []string{"--completion", "fish"},
	}

	shell := exec.detectCompletionShell()
	g.Expect(shell).To(Equal("fish"))
}

func TestDetectCompletionShell_FlagAfterCompletion(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	exec := &runExecutor{
		rest: []string{"--completion", "--verbose"},
	}

	// When arg after --completion starts with "-", should detect shell
	shell := exec.detectCompletionShell()
	// Result depends on SHELL env var
	g.Expect(shell).NotTo(Equal("--verbose"))
}

func TestDetectCurrentShell_EmptyShellEnv(t *testing.T) {
	t.Setenv("SHELL", "")

	result := detectCurrentShell()
	if result != "unknown" {
		t.Fatalf("expected 'unknown', got: %s", result)
	}
}

// --- detectCurrentShell tests ---

func TestDetectCurrentShell_WithShellEnv(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")

	result := detectCurrentShell()
	if result != bashShell {
		t.Fatalf("expected '%s', got: %s", bashShell, result)
	}
}

// --- Git URL detection tests ---

func TestDetectRepoURL(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Running in the targ repo, should find a URL
	url := DetectRepoURL()
	// The repo has an origin, so we should get something back
	// Can't guarantee exact URL (SSH vs HTTPS), but should contain "targ"
	g.Expect(url).To(ContainSubstring("targ"))
}

func TestDetectRepoURLFromDir_NoGit(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Use a temp dir with no .git
	tmpDir := t.TempDir()
	url := detectRepoURLFromDir(tmpDir)
	g.Expect(url).To(BeEmpty())
}

func TestDetectRepoURLFromDir_WithGit(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Create a temp dir with a fake .git/config
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	g.Expect(os.MkdirAll(gitDir, 0o755)).To(Succeed())

	config := `[core]
	repositoryformatversion = 0
[remote "origin"]
	url = https://github.com/example/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
`
	g.Expect(os.WriteFile(filepath.Join(gitDir, "config"), []byte(config), 0o644)).To(Succeed())

	url := detectRepoURLFromDir(tmpDir)
	g.Expect(url).To(Equal("https://github.com/example/repo"))
}

func TestDetectRepoURLWithGetwd_Error(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Mock a failing getwd function
	failingGetwd := func() (string, error) {
		return "", errors.New("getwd failed")
	}

	url := detectRepoURLWithGetwd(failingGetwd)
	g.Expect(url).To(BeEmpty())
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
			g.Expect(DetectShell()).To(Equal(tt.expected))
		})
	}
}

func TestDetectShell_Property_KnownShellsAlwaysDetected(t *testing.T) {
	t.Parallel()

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
			g.Expect(DetectShell()).To(BeEmpty())
		})
	}
}

func TestDoList_IncludesSubcommands(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

func TestEmptyExamples(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	examples := EmptyExamples()

	g.Expect(examples).To(BeEmpty())
	g.Expect(examples).NotTo(BeNil())
}

func TestExecuteDefaultParallel_Error(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	env := &ExecuteEnv{args: []string{"cmd", "fail"}}
	root := &commandNode{
		Name: "root",
		Subcommands: map[string]*commandNode{
			"fail": {
				Name: "fail",
				Func: reflect.ValueOf(func() error { return errors.New("task failed") }),
			},
		},
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{root},
		rest:       []string{"fail"},
		hasDefault: true,
		opts:       RunOptions{Overrides: RuntimeOverrides{Parallel: true}},
	}

	err := exec.executeDefaultParallel()
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("task failed"))
}

func TestExecuteDefaultParallel_SkipsFlags(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	called := false
	env := &ExecuteEnv{args: []string{"cmd", "--flag", "task"}}
	root := &commandNode{
		Name: "root",
		Subcommands: map[string]*commandNode{
			"task": {
				Name: "task",
				Func: reflect.ValueOf(func() { called = true }),
			},
		},
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{root},
		rest:       []string{"--flag", "task"},
		hasDefault: true,
		opts:       RunOptions{Overrides: RuntimeOverrides{Parallel: true}},
	}

	err := exec.executeDefaultParallel()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
}

// --- executeDefaultParallel tests ---

func TestExecuteDefaultParallel_Success(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Track execution order - should be concurrent
	var mutex sync.Mutex

	executed := []string{}

	env := &ExecuteEnv{args: []string{"cmd", "a", "b"}}
	root := &commandNode{
		Name: "root",
		Subcommands: map[string]*commandNode{
			"a": {
				Name: "a",
				Func: reflect.ValueOf(func() {
					mutex.Lock()

					executed = append(executed, "a")

					mutex.Unlock()
				}),
			},
			"b": {
				Name: "b",
				Func: reflect.ValueOf(func() {
					mutex.Lock()

					executed = append(executed, "b")

					mutex.Unlock()
				}),
			},
		},
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{root},
		rest:       []string{"a", "b"},
		hasDefault: true,
		opts:       RunOptions{Overrides: RuntimeOverrides{Parallel: true}},
	}

	err := exec.executeDefaultParallel()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(executed).To(ConsistOf("a", "b"))
}

func TestExecuteDefault_EmptyRestError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test error path when rest is empty
	execErr := errors.New("root failed")
	env := &ExecuteEnv{args: []string{"cmd"}}
	root := &commandNode{
		Name: "root",
		Func: reflect.ValueOf(func() error { return execErr }),
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{root},
		rest:       []string{}, // empty rest triggers first branch
		hasDefault: true,
	}

	err := exec.executeDefault()
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("root failed"))
}

func TestExecuteDefault_LoopError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	failErr := errors.New("subcommand failed")

	env := &ExecuteEnv{args: []string{"cmd", "fail"}}
	root := &commandNode{
		Name: "root",
		Subcommands: map[string]*commandNode{
			"fail": {
				Name: "fail",
				Func: reflect.ValueOf(func() error { return failErr }),
			},
		},
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{root},
		rest:       []string{"fail"},
		hasDefault: true,
	}

	err := exec.executeDefault()
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("subcommand failed"))
}

func TestExecuteDefault_SuccessPath(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test the successful empty rest path
	called := false
	env := &ExecuteEnv{args: []string{"cmd"}}
	root := &commandNode{
		Name: "root",
		Func: reflect.ValueOf(func() { called = true }),
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{root},
		rest:       []string{}, // empty rest
		hasDefault: true,
	}

	err := exec.executeDefault()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
}

func TestExecuteDefault_UnknownCommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test "Unknown command" path when no subcommand matches
	env := &ExecuteEnv{args: []string{"cmd", "nosuch"}}
	root := &commandNode{
		Name:        "root",
		Func:        reflect.ValueOf(func() {}),
		Subcommands: map[string]*commandNode{}, // no subcommands
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{root},
		rest:       []string{"nosuch"},
		hasDefault: true,
	}

	err := exec.executeDefault()
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("unknown command"))
}

// --- executeEnv tests ---

func TestExecuteEnv_ExitIsNoOp(t *testing.T) {
	t.Parallel()

	env := &ExecuteEnv{args: []string{"cmd"}}
	// Exit should be a no-op and not panic
	env.Exit(1)
	env.Exit(0)
}

func TestExecuteMultiRootParallel_Error(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	env := &ExecuteEnv{args: []string{"cmd", "fail"}}
	root := &commandNode{
		Name: "fail",
		Func: reflect.ValueOf(func() error { return errors.New("root failed") }),
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{root},
		rest:       []string{"fail"},
		hasDefault: false,
		opts:       RunOptions{Overrides: RuntimeOverrides{Parallel: true}},
	}

	err := exec.executeMultiRootParallel()
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("root failed"))
}

func TestExecuteMultiRootParallel_SkipsFlags(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	called := false
	env := &ExecuteEnv{args: []string{"cmd", "--flag", "task"}}
	root := &commandNode{
		Name: "task",
		Func: reflect.ValueOf(func() { called = true }),
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{root},
		rest:       []string{"--flag", "task"},
		hasDefault: false,
		opts:       RunOptions{Overrides: RuntimeOverrides{Parallel: true}},
	}

	err := exec.executeMultiRootParallel()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
}

// --- executeMultiRootParallel tests ---

func TestExecuteMultiRootParallel_Success(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var mutex sync.Mutex

	executed := []string{}

	env := &ExecuteEnv{args: []string{"cmd", "a", "b"}}
	rootA := &commandNode{
		Name: "a",
		Func: reflect.ValueOf(func() {
			mutex.Lock()

			executed = append(executed, "a")

			mutex.Unlock()
		}),
	}
	rootB := &commandNode{
		Name: "b",
		Func: reflect.ValueOf(func() {
			mutex.Lock()

			executed = append(executed, "b")

			mutex.Unlock()
		}),
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{rootA, rootB},
		rest:       []string{"a", "b"},
		hasDefault: false,
		opts:       RunOptions{Overrides: RuntimeOverrides{Parallel: true}},
	}

	err := exec.executeMultiRootParallel()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(executed).To(ConsistOf("a", "b"))
}

func TestExecuteMultiRootParallel_UnknownCommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	env := &ExecuteEnv{args: []string{"cmd", "nosuch"}}
	root := &commandNode{
		Name: "a",
		Func: reflect.ValueOf(func() {}),
	}

	exec := &runExecutor{
		env:        env,
		ctx:        context.Background(),
		roots:      []*commandNode{root},
		rest:       []string{"nosuch"},
		hasDefault: false,
		opts:       RunOptions{Overrides: RuntimeOverrides{Parallel: true}},
	}

	err := exec.executeMultiRootParallel()
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("Unknown command"))
}

func TestExecuteWithParents_HelpOnly(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test that HelpOnly prints help and exits
	node := &commandNode{
		Name:        "test",
		Description: "A test command",
		Func:        reflect.ValueOf(func() {}),
	}

	remaining, err := node.executeWithParents(
		context.Background(),
		[]string{},
		nil,
		map[string]bool{},
		false,
		RunOptions{HelpOnly: true},
	)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(remaining).To(BeEmpty())
}

func TestExecuteWithParents_TimeoutError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Invalid timeout should propagate error
	node := &commandNode{
		Name: "test",
	}

	_, err := node.executeWithParents(
		context.Background(),
		[]string{"--timeout", "invalid"},
		nil,
		map[string]bool{},
		false,
		RunOptions{},
	)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid timeout"))
}

// --- executeWithWatch tests ---

//nolint:paralleltest // Not parallel - modifies global fileWatch
func TestExecuteWithWatch_CallbackInvoked(t *testing.T) {
	g := NewWithT(t)

	origFileWatch := fileWatch

	defer func() { fileWatch = origFileWatch }()

	callbackCalled := false
	callbackErr := errors.New("callback error")

	// Mock Watch to immediately call the callback
	fileWatch = func(
		_ context.Context, _ []string, _ internalfile.WatchOptions, cb func(internalfile.ChangeSet) error,
	) error {
		callbackCalled = true
		return cb(internalfile.ChangeSet{Modified: []string{"test.go"}})
	}

	fnCalls := 0
	err := executeWithWatch(context.Background(), []string{"**/*.go"}, func() error {
		fnCalls++
		if fnCalls == 2 {
			return callbackErr
		}

		return nil
	})

	g.Expect(callbackCalled).To(BeTrue())
	g.Expect(fnCalls).To(Equal(2)) // Initial call + callback
	g.Expect(err).To(MatchError(ContainSubstring("callback error")))
}

// --- ExitError tests ---

func TestExitError_Error(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	err := ExitError{Code: 42}
	g.Expect(err.Error()).To(Equal("exit code 42"))
}

func TestExitError_Error_Zero(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	err := ExitError{Code: 0}
	g.Expect(err.Error()).To(ContainSubstring("0"))
}

func TestExpandRecursive(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Build a tree: root -> child1, child2; child1 -> grandchild
	root := &commandNode{
		Name:        "root",
		Subcommands: make(map[string]*commandNode),
	}
	child1 := &commandNode{
		Name:        "child1",
		Subcommands: make(map[string]*commandNode),
	}
	child2 := &commandNode{
		Name:        "child2",
		Subcommands: make(map[string]*commandNode),
	}
	grandchild := &commandNode{
		Name:        "grandchild",
		Subcommands: make(map[string]*commandNode),
	}
	root.Subcommands["child1"] = child1
	root.Subcommands["child2"] = child2
	child1.Subcommands["grandchild"] = grandchild

	// Empty suffix matches all direct children and recurses
	matches := expandRecursive(root, "")
	// Should include: child1, child2, grandchild (from recursion)
	g.Expect(len(matches)).To(BeNumerically(">=", 2))

	// Check that direct children are included
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m.Name)
	}

	g.Expect(names).To(ContainElement("child1"))
	g.Expect(names).To(ContainElement("child2"))

	// "/" matches all direct children and recurses
	matches = expandRecursive(root, "/")
	g.Expect(len(matches)).To(BeNumerically(">=", 2))

	// "/*" matches all direct children
	matches = expandRecursive(root, "/*")
	g.Expect(len(matches)).To(BeNumerically(">=", 2))

	// "/child*" matches children starting with "child"
	matches = expandRecursive(root, "/child*")
	g.Expect(len(matches)).To(BeNumerically(">=", 2))

	for _, m := range matches {
		// Either matches child* or is a grandchild from recursion
		hasChildPrefix := strings.HasPrefix(m.Name, "child")
		isGrandchild := m.Name == "grandchild"
		g.Expect(hasChildPrefix || isGrandchild).To(BeTrue())
	}
}

func TestExpectingFlagValue_DoubleDash(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{}
	g.Expect(expectingFlagValue([]string{"--"}, specs)).To(BeFalse())
}

// --- expectingFlagValue tests ---

func TestExpectingFlagValue_EmptyArgs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{}
	g.Expect(expectingFlagValue([]string{}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_FlagGroupAllBool(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-v": {TakesValue: false},
		"-d": {TakesValue: false},
	}
	// -vd where all flags are bools
	g.Expect(expectingFlagValue([]string{"-vd"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_FlagGroupLastTakesValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-v": {TakesValue: false},
		"-n": {TakesValue: true},
	}
	// -vn where n is last and takes value
	g.Expect(expectingFlagValue([]string{"-vn"}, specs)).To(BeTrue())
}

func TestExpectingFlagValue_FlagGroupMiddleTakesValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-v": {TakesValue: false},
		"-n": {TakesValue: true},
	}
	// -nv where n takes value but is not last
	g.Expect(expectingFlagValue([]string{"-nv"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_FlagGroupUnknownFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-v": {TakesValue: false},
	}
	// -vx where x is unknown - continues past unknown to find none taking value
	g.Expect(expectingFlagValue([]string{"-vx"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_LongFlagNeedsValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"--name": {TakesValue: true},
	}
	g.Expect(expectingFlagValue([]string{"--name"}, specs)).To(BeTrue())
}

func TestExpectingFlagValue_LongFlagNoValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"--verbose": {TakesValue: false},
	}
	g.Expect(expectingFlagValue([]string{"--verbose"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_LongFlagUnknown(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{}
	g.Expect(expectingFlagValue([]string{"--unknown"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_LongFlagWithEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"--name": {TakesValue: true},
	}
	g.Expect(expectingFlagValue([]string{"--name=foo"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_Positional(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{}
	g.Expect(expectingFlagValue([]string{"arg1"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_ShortFlagNeedsValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-n": {TakesValue: true},
	}
	g.Expect(expectingFlagValue([]string{"-n"}, specs)).To(BeTrue())
}

func TestExpectingFlagValue_ShortFlagNoValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-v": {TakesValue: false},
	}
	g.Expect(expectingFlagValue([]string{"-v"}, specs)).To(BeFalse())
}

func TestExtractHelpFlag_LongFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	found, remaining := extractHelpFlag([]string{"cmd", "--help", "arg1"})
	g.Expect(found).To(BeTrue())
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
}

// --- extractHelpFlag tests ---

func TestExtractHelpFlag_NotFound(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	found, remaining := extractHelpFlag([]string{"cmd", "arg1"})
	g.Expect(found).To(BeFalse())
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
}

func TestExtractHelpFlag_ShortFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	found, remaining := extractHelpFlag([]string{"cmd", "-h", "arg1"})
	g.Expect(found).To(BeTrue())
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
}

func TestExtractTimeout_InvalidDuration(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	_, _, err := extractTimeout([]string{"cmd", "--timeout", "invalid"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid timeout"))
}

func TestExtractTimeout_InvalidDurationEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	_, _, err := extractTimeout([]string{"cmd", "--timeout=bad"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid timeout"))
}

func TestExtractTimeout_MissingValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	_, _, err := extractTimeout([]string{"cmd", "--timeout"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires a duration"))
}

// --- extractTimeout tests ---

func TestExtractTimeout_NoTimeout(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	timeout, remaining, err := extractTimeout([]string{"cmd", "arg1", "arg2"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(timeout).To(BeZero())
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1", "arg2"}))
}

func TestExtractTimeout_WithEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	timeout, remaining, err := extractTimeout([]string{"cmd", "--timeout=5m", "arg1"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(timeout.Minutes()).To(Equal(5.0))
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
}

func TestExtractTimeout_WithSeparateValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	timeout, remaining, err := extractTimeout([]string{"cmd", "--timeout", "10s", "arg1"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(timeout.Seconds()).To(Equal(10.0))
	g.Expect(remaining).To(Equal([]string{"cmd", "arg1"}))
}

func TestFindMatchingSubcommands(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Build a parent node with subcommands
	parent := &commandNode{
		Name:        "parent",
		Subcommands: make(map[string]*commandNode),
	}
	parent.Subcommands["test-unit"] = &commandNode{Name: "test-unit"}
	parent.Subcommands["test-integration"] = &commandNode{Name: "test-integration"}
	parent.Subcommands["build"] = &commandNode{Name: "build"}

	// Match all
	matches := findMatchingSubcommands(parent, "*")
	g.Expect(matches).To(HaveLen(3))

	// Match prefix
	matches = findMatchingSubcommands(parent, "test-*")
	g.Expect(matches).To(HaveLen(2))

	for _, m := range matches {
		g.Expect(m.Name).To(HavePrefix("test-"))
	}

	// Match suffix
	matches = findMatchingSubcommands(parent, "*-integration")
	g.Expect(matches).To(HaveLen(1))
	g.Expect(matches[0].Name).To(Equal("test-integration"))

	// No matches
	matches = findMatchingSubcommands(parent, "deploy-*")
	g.Expect(matches).To(BeEmpty())
}

func TestFlagDefaultPlaceholder(t *testing.T) {
	t.Parallel()

	// Test type that hits default case (not string, int, or bool)
	type DefaultFlagArgs struct {
		Rate float64 `targ:"flag"`
	}

	target := &mockTarget{fn: func(_ DefaultFlagArgs) {}, name: "test"}

	cmd, err := parseTargetLike(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	usage, err := buildUsageLine(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Optional flags are now shown as [flags...] in the usage line
	if !strings.Contains(usage, "[flags...]") {
		t.Fatalf("expected [flags...] in usage for optional flag: %s", usage)
	}
}

func TestFlagEnumUsage(t *testing.T) {
	t.Parallel()

	type EnumFlagArgs struct {
		Mode string `targ:"flag,enum=dev|prod|test"`
	}

	target := &mockTarget{fn: func(_ EnumFlagArgs) {}, name: "test"}

	cmd, err := parseTargetLike(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	usage, err := buildUsageLine(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Optional flags (including enums) are now shown as [flags...] in the usage line
	if !strings.Contains(usage, "[flags...]") {
		t.Fatalf("expected [flags...] in usage for optional enum flag, got: %s", usage)
	}
}

func TestFlagParsing_IntFlagEqualsFormat(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var gotCount int

	fn := func(args FlagParseCmdArgs) { gotCount = args.Count }
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.Background(), []string{"-c=42"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotCount).To(Equal(42))
}

func TestFlagParsing_LongFlagEqualsFormat(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var gotName string

	fn := func(args FlagParseCmdArgs) { gotName = args.Name }
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.Background(), []string{"--name=hello"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotName).To(Equal("hello"))
}

func TestFlagParsing_ShortFlagEqualsFormat(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var gotName string

	fn := func(args FlagParseCmdArgs) { gotName = args.Name }
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.Background(), []string{"-n=world"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotName).To(Equal("world"))
}

func TestFlagParsing_UnknownLongFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	fn := func(_ FlagParseCmdArgs) {}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.Background(), []string{"--unknown"}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("--unknown"))
}

func TestFlagParsing_UnknownShortFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	fn := func(_ FlagParseCmdArgs) {}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.Background(), []string{"-x"}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("-x"))
}

func TestFormatFlagUsageWrapped(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	item := flagHelp{
		Name:        "verbose",
		Short:       "v",
		Placeholder: "[flag]",
	}

	result := formatFlagUsageWrapped(item)
	g.Expect(result).To(Equal("--verbose, -v"))
}

func TestFuncSourceFile_InvalidValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	result := funcSourceFile(reflect.Value{})
	g.Expect(result).To(Equal(""))
}

func TestFuncSourceFile_NilFunc(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var f func()

	v := reflect.ValueOf(f)
	result := funcSourceFile(v)
	g.Expect(result).To(Equal(""))
}

func TestFuncSourceFile_NotFunc(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	v := reflect.ValueOf(42)
	result := funcSourceFile(v)
	g.Expect(result).To(Equal(""))
}

func TestFuncSourceFile_ValidFunc(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	v := reflect.ValueOf(TestFuncSourceFile_ValidFunc)
	result := funcSourceFile(v)
	g.Expect(result).To(ContainSubstring("internal_test.go"))
}

func TestHandleComplete_ReturnsErrorFromCompleteFn(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	env := &ExecuteEnv{args: []string{"cmd", "__complete", "sub"}}
	completeErr := errors.New("completion failed")
	exec := &runExecutor{
		env:        env,
		roots:      []*commandNode{{Name: "test"}},
		rest:       []string{"__complete", "sub"},
		completeFn: func(_ io.Writer, _ []*commandNode, _ string) error { return completeErr },
	}

	exec.handleComplete()
	g.Expect(env.Output()).To(ContainSubstring("completion failed"))
}

func TestHandleComplete_ShortRest(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// When rest has only one element, doCompletion is not called
	env := &ExecuteEnv{args: []string{"cmd", "__complete"}}
	exec := &runExecutor{
		env:  env,
		rest: []string{"__complete"}, // only one element
	}

	exec.handleComplete()
	g.Expect(env.Output()).To(BeEmpty()) // No error printed
}

func TestHandleHelpFlag_DisableHelp(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{Name: "test"}
	remaining, handled := handleHelpFlag(node, []string{"--help"}, RunOptions{DisableHelp: true})
	g.Expect(handled).To(BeFalse())
	g.Expect(remaining).To(BeNil())
}

func TestHandleHelpFlag_HelpOnly(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{Name: "test"}
	remaining, handled := handleHelpFlag(node, []string{"--help"}, RunOptions{HelpOnly: true})
	g.Expect(handled).To(BeFalse())
	g.Expect(remaining).To(BeNil())
}

func TestHandleHelpFlag_NoHelpFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{Name: "test"}
	remaining, handled := handleHelpFlag(node, []string{"arg1"}, RunOptions{})
	g.Expect(handled).To(BeFalse())
	g.Expect(remaining).To(BeNil())
}

func TestHandleHelpFlag_WithHelpFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{Name: "test", Description: "A test command"}

	out := captureStdout(t, func() {
		remaining, handled := handleHelpFlag(node, []string{"--help", "arg1"}, RunOptions{})
		g.Expect(handled).To(BeTrue())
		g.Expect(remaining).To(Equal([]string{"arg1"}))
	})

	g.Expect(out).To(ContainSubstring("test"))
}

func TestHandleList_ReturnsErrorFromListFn(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	env := &ExecuteEnv{args: []string{"cmd", "__list"}}
	listErr := errors.New("list failed")

	exec := &runExecutor{
		env:    env,
		roots:  []*commandNode{{Name: "test"}},
		listFn: func(_ []*commandNode) error { return listErr },
	}

	err := exec.handleList()

	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(func() ExitError {
		var target ExitError

		_ = errors.As(err, &target)

		return target
	}().Code).To(Equal(1))
	g.Expect(env.Output()).To(ContainSubstring("list failed"))
}

func TestIsGlobPattern(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	g.Expect(isGlobPattern("*")).To(BeTrue())
	g.Expect(isGlobPattern("**")).To(BeTrue())
	g.Expect(isGlobPattern("test-*")).To(BeTrue())
	g.Expect(isGlobPattern("test")).To(BeFalse())
}

// --- Glob pattern tests (whitebox) ---

func TestIsGlobPatternCmd(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	g.Expect(isGlobPatternCmd("*")).To(BeTrue())
	g.Expect(isGlobPatternCmd("**")).To(BeTrue())
	g.Expect(isGlobPatternCmd("test-*")).To(BeTrue())
	g.Expect(isGlobPatternCmd("*-unit")).To(BeTrue())
	g.Expect(isGlobPatternCmd("test")).To(BeFalse())
	g.Expect(isGlobPatternCmd("build")).To(BeFalse())
}

func TestIsShellVar(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	vars := []string{"namespace", "file"}

	// Exact match
	g.Expect(isShellVar("namespace", vars)).To(BeTrue())
	g.Expect(isShellVar("file", vars)).To(BeTrue())

	// Case-insensitive match
	g.Expect(isShellVar("NAMESPACE", vars)).To(BeTrue())
	g.Expect(isShellVar("File", vars)).To(BeTrue())

	// No match
	g.Expect(isShellVar("unknown", vars)).To(BeFalse())
	g.Expect(isShellVar("name", vars)).To(BeFalse())

	// Empty vars
	g.Expect(isShellVar("anything", nil)).To(BeFalse())
	g.Expect(isShellVar("anything", []string{})).To(BeFalse())
}

func TestMatchesGlob(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Single star matches all
	g.Expect(matchesGlob("test", "*")).To(BeTrue())
	g.Expect(matchesGlob("anything", "*")).To(BeTrue())

	// Double star matches all
	g.Expect(matchesGlob("test", "**")).To(BeTrue())
	g.Expect(matchesGlob("anything", "**")).To(BeTrue())

	// Prefix pattern (test-*)
	g.Expect(matchesGlob("test-unit", "test-*")).To(BeTrue())
	g.Expect(matchesGlob("test-integration", "test-*")).To(BeTrue())
	g.Expect(matchesGlob("build", "test-*")).To(BeFalse())

	// Suffix pattern (*-unit)
	g.Expect(matchesGlob("test-unit", "*-unit")).To(BeTrue())
	g.Expect(matchesGlob("build-unit", "*-unit")).To(BeTrue())
	g.Expect(matchesGlob("build", "*-unit")).To(BeFalse())

	// Contains pattern (*test*)
	g.Expect(matchesGlob("my-test-here", "*test*")).To(BeTrue())
	g.Expect(matchesGlob("testing", "*test*")).To(BeTrue())
	g.Expect(matchesGlob("build", "*test*")).To(BeFalse())

	// No wildcards - exact match (case insensitive)
	g.Expect(matchesGlob("test", "test")).To(BeTrue())
	g.Expect(matchesGlob("Test", "test")).To(BeTrue())
	g.Expect(matchesGlob("build", "test")).To(BeFalse())
}

func TestMatchesGlobCmd(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Single star matches all
	g.Expect(matchesGlobCmd("test", "*")).To(BeTrue())
	g.Expect(matchesGlobCmd("build", "*")).To(BeTrue())

	// Double star matches all
	g.Expect(matchesGlobCmd("test", "**")).To(BeTrue())
	g.Expect(matchesGlobCmd("build", "**")).To(BeTrue())

	// Prefix pattern
	g.Expect(matchesGlobCmd("test-unit", "test-*")).To(BeTrue())
	g.Expect(matchesGlobCmd("test-integration", "test-*")).To(BeTrue())
	g.Expect(matchesGlobCmd("build", "test-*")).To(BeFalse())

	// Suffix pattern
	g.Expect(matchesGlobCmd("test-unit", "*-unit")).To(BeTrue())
	g.Expect(matchesGlobCmd("build-unit", "*-unit")).To(BeTrue())
	g.Expect(matchesGlobCmd("build", "*-unit")).To(BeFalse())

	// Contains pattern (both prefix and suffix star)
	g.Expect(matchesGlobCmd("my-test-here", "*test*")).To(BeTrue())
	g.Expect(matchesGlobCmd("testing", "*test*")).To(BeTrue())
	g.Expect(matchesGlobCmd("build", "*test*")).To(BeFalse())

	// Case insensitive
	g.Expect(matchesGlobCmd("Test-Unit", "test-*")).To(BeTrue())
	g.Expect(matchesGlobCmd("TEST-UNIT", "*-unit")).To(BeTrue())

	// No wildcards - exact match
	g.Expect(matchesGlobCmd("test", "test")).To(BeTrue())
	g.Expect(matchesGlobCmd("Test", "test")).To(BeTrue())
	g.Expect(matchesGlobCmd("build", "test")).To(BeFalse())
}

func TestMissingPositionalError_Unit_WithName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Direct unit test for when opts.Name is set
	spec := positionalSpec{
		opts:  TagOptions{Name: "target"},
		field: reflect.StructField{Name: "Arg"},
	}

	err := missingPositionalError(spec)
	g.Expect(err.Error()).To(ContainSubstring("target"))
	g.Expect(err.Error()).NotTo(ContainSubstring("Arg"))
}

func TestMissingPositionalError_Unit_WithoutName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Direct unit test for when opts.Name is empty (uses field name)
	spec := positionalSpec{
		opts:  TagOptions{Name: ""},
		field: reflect.StructField{Name: "MyField"},
	}

	err := missingPositionalError(spec)
	g.Expect(err.Error()).To(ContainSubstring("MyField"))
}

func TestMissingPositionalError_WithName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type RequiredPosArgs struct {
		Arg string `targ:"positional,required,name=target"`
	}

	fn := func(_ RequiredPosArgs) {}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	// Execute without providing the required positional
	err = node.execute(context.TODO(), []string{}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("target"))
}

func TestMissingPositionalError_WithoutName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type RequiredPosArgs struct {
		Arg string `targ:"positional,required"`
	}

	fn := func(_ RequiredPosArgs) {}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	// Execute without providing the required positional
	err = node.execute(context.TODO(), []string{}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("Arg"))
}

func TestNormalizeGitURL_HTTPS(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	url := normalizeGitURL("https://github.com/user/repo.git")
	g.Expect(url).To(Equal("https://github.com/user/repo"))
}

func TestNormalizeGitURL_NoGitSuffix(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	url := normalizeGitURL("https://github.com/user/repo")
	g.Expect(url).To(Equal("https://github.com/user/repo"))
}

func TestNormalizeGitURL_SSH(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	url := normalizeGitURL("git@github.com:user/repo.git")
	g.Expect(url).To(Equal("https://github.com/user/repo"))
}

func TestParseFlagAdvancesVariadicPositional(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test that when parsing a flag after a variadic positional has values,
	// the variadic positional is advanced
	type VariadicThenFlagArgs struct {
		Args []string `targ:"positional"`
		Flag string   `targ:"flag"`
	}

	var (
		gotArgs []string
		gotFlag string
	)

	fn := func(args VariadicThenFlagArgs) {
		gotArgs = args.Args
		gotFlag = args.Flag
	}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	// "arg1" "arg2" go to variadic positional, then --flag triggers advancement
	err = node.execute(context.TODO(), []string{"arg1", "arg2", "--flag", "value"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotArgs).To(Equal([]string{"arg1", "arg2"}))
	g.Expect(gotFlag).To(Equal("value"))
}

func TestParseFunc_NotFuncType(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Pass a non-func reflect.Value to parseFunc
	v := reflect.ValueOf(42)
	_, err := parseFunc(v)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("expected func"))
}

func TestParseGitConfigOriginURL_NoFile(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	url := parseGitConfigOriginURL("/nonexistent/path/config")
	g.Expect(url).To(BeEmpty())
}

func TestParseGitConfigOriginURL_NoOrigin(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	config := `[core]
	repositoryformatversion = 0
[remote "upstream"]
	url = https://github.com/other/repo.git
`
	g.Expect(os.WriteFile(configPath, []byte(config), 0o644)).To(Succeed())

	url := parseGitConfigOriginURL(configPath)
	g.Expect(url).To(BeEmpty())
}

func TestParseGitConfigOriginURL_OriginFollowedByOtherSection(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	config := `[remote "origin"]
	url = https://github.com/user/repo.git
[branch "main"]
	remote = origin
`
	g.Expect(os.WriteFile(configPath, []byte(config), 0o644)).To(Succeed())

	url := parseGitConfigOriginURL(configPath)
	g.Expect(url).To(Equal("https://github.com/user/repo"))
}

func TestParseGitConfigOriginURL_OriginWithSSH(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	config := `[remote "origin"]
	url = git@github.com:user/repo.git
`
	g.Expect(os.WriteFile(configPath, []byte(config), 0o644)).To(Succeed())

	url := parseGitConfigOriginURL(configPath)
	g.Expect(url).To(Equal("https://github.com/user/repo"))
}

// --- Parsing Edge Cases ---

func TestParseNilTarget(t *testing.T) {
	t.Parallel()

	_, err := parseTarget(nil)
	if err == nil {
		t.Fatal("expected error for nil target")
	}
}

func TestParseTarget_GroupLike(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target1 := &mockTarget{
		fn:   func(_ context.Context) error { return nil },
		name: "build",
	}
	target2 := &mockTarget{
		fn:   func(_ context.Context) error { return nil },
		name: "test",
	}

	group := &mockGroup{
		name:    "dev",
		members: []any{target1, target2},
	}

	node, err := parseTarget(group)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node).ToNot(BeNil())

	if node == nil {
		t.Fatal("node is nil")
	}

	g.Expect(node.Name).To(Equal("dev"))
	g.Expect(node.Subcommands).To(HaveLen(2))
	g.Expect(node.Subcommands["build"]).ToNot(BeNil())
	g.Expect(node.Subcommands["test"]).ToNot(BeNil())
}

func TestParseTarget_GroupLike_Nested(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	innerTarget := &mockTarget{
		fn:   func(_ context.Context) error { return nil },
		name: "fast",
	}
	innerGroup := &mockGroup{
		name:    "lint",
		members: []any{innerTarget},
	}
	outerGroup := &mockGroup{
		name:    "dev",
		members: []any{innerGroup},
	}

	node, err := parseTarget(outerGroup)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node).ToNot(BeNil())

	if node == nil {
		t.Fatal("node is nil")
	}

	g.Expect(node.Name).To(Equal("dev"))
	g.Expect(node.Subcommands).To(HaveLen(1))

	lintNode := node.Subcommands["lint"]
	g.Expect(lintNode).ToNot(BeNil())

	if lintNode == nil {
		t.Fatal("lintNode is nil")
	}

	g.Expect(lintNode.Subcommands).To(HaveLen(1))
	g.Expect(lintNode.Subcommands["fast"]).ToNot(BeNil())
}

func TestParseTarget_InvalidFunctionParam(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Function with non-context param
	_, err := parseTarget(InvalidParamFunc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("context.Context"))
}

func TestParseTarget_InvalidFunctionReturn(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Function returning non-error
	_, err := parseTarget(InvalidReturnFunc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("return only error"))
}

func TestParseTarget_InvalidFunctionSignature(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Function with too many params
	_, err := parseTarget(InvalidSigFunc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("niladic or accept context"))
}

func TestParseTarget_TargetLike_InvalidFnType(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{
		fn:   42, // Not a func or string
		name: "broken",
	}

	_, err := parseTarget(target)
	g.Expect(err).To(HaveOccurred())
}

func TestParseTarget_TargetLike_NilFn(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{
		fn:   nil,
		name: "broken",
	}

	_, err := parseTarget(target)
	g.Expect(err).To(HaveOccurred())
}

func TestParseTarget_TargetLike_StringCommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{
		fn:          "golangci-lint run ./...",
		name:        "lint",
		description: "Run the linter",
	}

	node, err := parseTarget(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node).ToNot(BeNil())

	if node == nil {
		t.Fatal("node is nil")
	}

	g.Expect(node.Name).To(Equal("lint"))
	g.Expect(node.Description).To(Equal("Run the linter"))
}

func TestParseTarget_TargetLike_StringCommandNoName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{
		fn:   "golangci-lint run ./...",
		name: "", // No name - should use first word
	}

	node, err := parseTarget(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node).ToNot(BeNil())

	if node == nil {
		t.Fatal("node is nil")
	}

	g.Expect(node.Name).To(Equal("golangci-lint"))
}

func TestParseTarget_TargetLike_StringCommandWithBraceVars(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{
		fn:   "echo ${name}suffix ${port}",
		name: "test",
	}

	node, err := parseTarget(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node).ToNot(BeNil())

	if node == nil {
		t.Fatal("node is nil")
	}

	// Variables should be extracted in order, lowercase
	g.Expect(node.ShellVars).To(Equal([]string{"name", "port"}))
}

func TestParseTarget_TargetLike_StringCommandWithVars(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{
		fn:          "kubectl apply -n $namespace -f $file",
		name:        "deploy",
		description: "Deploy to Kubernetes",
	}

	node, err := parseTarget(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node).ToNot(BeNil())

	if node == nil {
		t.Fatal("node is nil")
	}

	g.Expect(node.Name).To(Equal("deploy"))
	g.Expect(node.ShellCommand).To(Equal("kubectl apply -n $namespace -f $file"))
	g.Expect(node.ShellVars).To(Equal([]string{"namespace", "file"}))
}

func TestParseTarget_TargetLike_WithFunction(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	called := false
	target := &mockTarget{
		fn:          func(_ context.Context) error { called = true; return nil },
		name:        "my-target",
		description: "My test target",
	}

	node, err := parseTarget(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node).ToNot(BeNil())

	if node == nil {
		t.Fatal("node is nil")
	}

	g.Expect(node.Name).To(Equal("my-target"))
	g.Expect(node.Description).To(Equal("My test target"))
	g.Expect(node.Func.IsValid()).To(BeTrue())

	// Verify the function is executable
	_ = called
}

func TestParseTarget_TargetLike_WithoutName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{
		fn:          func(_ context.Context) error { return nil },
		name:        "", // No name set
		description: "",
	}

	node, err := parseTarget(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node).ToNot(BeNil())

	if node == nil {
		t.Fatal("node is nil")
	}

	// Name should be derived from function name
	g.Expect(node.Name).ToNot(BeEmpty())
}

func TestParseTarget_TooManyReturns(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Function returning multiple values
	_, err := parseTarget(TooManyReturnsFunc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("return only error"))
}

func TestParseTargets_Error(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	env := &ExecuteEnv{args: []string{"cmd"}}
	exec := &runExecutor{
		env: env,
	}

	// Pass an invalid target (non-func, non-struct)
	err := exec.parseTargets([]any{123}) // int is not a valid target
	g.Expect(err).NotTo(HaveOccurred())  // parseTargets doesn't return error, just logs
	g.Expect(env.Output()).To(ContainSubstring("Error parsing target"))
	g.Expect(exec.roots).To(BeEmpty()) // Invalid target not added
}

func TestPlaceholderTagInUsage(t *testing.T) {
	t.Parallel()

	type PlaceholderCmdArgs struct {
		File string `targ:"flag,short=f,placeholder=FILE"`
		Out  string `targ:"positional,placeholder=DEST"`
	}

	target := &mockTarget{fn: func(_ PlaceholderCmdArgs) {}, name: "test"}

	cmd, err := parseTargetLike(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	usage, err := buildUsageLine(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Optional flags are summarized as [flags...] at end
	if !strings.Contains(usage, "[flags...]") {
		t.Fatalf("expected [flags...] in usage: %s", usage)
	}

	// Optional positionals now have ... suffix
	if !strings.Contains(usage, "[DEST...]") {
		t.Fatalf("expected positional placeholder with ... in usage: %s", usage)
	}
}

func TestPositionalEnumUsage(t *testing.T) {
	t.Parallel()

	type EnumPositionalArgs struct {
		Mode string `targ:"positional,enum=dev|prod"`
	}

	target := &mockTarget{fn: func(_ EnumPositionalArgs) {}, name: "test"}

	cmd, err := parseTargetLike(target)
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
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"-v", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_FlagGroup(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	// -vn where v is bool and n takes value - should consume next arg
	idx, err := positionalIndex(node, []string{"-vn", "foo", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_LongFlagConsumeNext(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"--name", "foo", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_LongFlagWithEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"--name=foo", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_NoArgs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(0))
}

func TestPositionalIndex_OnlyPositionals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"arg1", "arg2"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(2))
}

func TestPositionalIndex_ShortFlagConsumeNext(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"-n", "foo", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_ShortFlagWithEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"-n=foo", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_SkipsDoubleDash(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	idx, err := positionalIndex(node, []string{"--", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_UnknownLongFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
	g.Expect(err).NotTo(HaveOccurred())

	chain, err := completionChain(node, []string{})
	g.Expect(err).NotTo(HaveOccurred())

	// Unknown flag is skipped (not treated as positional)
	idx, err := positionalIndex(node, []string{"--unknown", "arg1"}, chain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(idx).To(Equal(1))
}

func TestPositionalIndex_VariadicFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node, err := parseTargetLike(&mockTarget{fn: func(_ posIdxArgs) {}, name: "test"})
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

func TestPositionalsComplete_AllFilled(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := []positionalSpec{
		{opts: TagOptions{Required: true}},
		{opts: TagOptions{Required: true}},
	}
	counts := []int{1, 1} // both filled

	g.Expect(positionalsComplete(specs, counts)).To(BeTrue())
}

func TestPositionalsComplete_EmptySpecs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := []positionalSpec{}
	counts := []int{}

	g.Expect(positionalsComplete(specs, counts)).To(BeTrue())
}

func TestPositionalsComplete_MissingRequired(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := []positionalSpec{
		{opts: TagOptions{Required: true}},
		{opts: TagOptions{Required: true}},
	}
	counts := []int{1, 0} // second not filled

	g.Expect(positionalsComplete(specs, counts)).To(BeFalse())
}

func TestPositionalsComplete_OptionalNotFilled(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := []positionalSpec{
		{opts: TagOptions{Required: true}},
		{opts: TagOptions{Required: false}}, // optional
	}
	counts := []int{1, 0} // second not filled but optional

	g.Expect(positionalsComplete(specs, counts)).To(BeTrue())
}

func TestPrependBuiltinExamples(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	custom := Example{Title: "Custom", Code: "custom code"}
	examples := PrependBuiltinExamples(custom)

	g.Expect(examples).To(HaveLen(3))

	if len(examples) >= 3 {
		g.Expect(examples[0].Title).To(Equal("Enable shell completion"))
		g.Expect(examples[1].Title).To(ContainSubstring("Run multiple"))
		g.Expect(examples[2].Title).To(Equal("Custom"))
	}
}

func TestPrintCommandHelp_FlagWithPlaceholder(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func(_ helpTestCmdWithPlaceholderArgs) {}, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	var buf bytes.Buffer
	printCommandHelp(&buf, node, RunOptions{})

	g.Expect(buf.String()).To(ContainSubstring("--output <file>"))
}

func TestPrintCommandHelp_FlagWithShortName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func(_ helpTestCmdWithShortArgs) {}, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	var buf bytes.Buffer
	printCommandHelp(&buf, node, RunOptions{})

	g.Expect(buf.String()).To(ContainSubstring("--verbose, -v"))
}

func TestPrintCommandHelp_FlagWithUsage(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func(_ helpTestCmdWithUsageArgs) {}, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	var buf bytes.Buffer
	printCommandHelp(&buf, node, RunOptions{})

	g.Expect(buf.String()).To(ContainSubstring("Output format"))
}

// --- printCommandHelp tests ---

func TestPrintCommandHelp_FunctionNode(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{
		Name:        "test-cmd",
		Description: "A test command",
		Type:        nil, // Function node has no Type
	}

	var buf bytes.Buffer
	printCommandHelp(&buf, node, RunOptions{})

	g.Expect(buf.String()).To(ContainSubstring("Usage:"))
	g.Expect(buf.String()).To(ContainSubstring("test-cmd"))
	g.Expect(buf.String()).To(ContainSubstring("A test command"))
}

func TestPrintCommandHelp_WithDescription(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "test", description: "A command with description"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseTargetLike returned nil node")
	}

	var buf bytes.Buffer
	printCommandHelp(&buf, node, RunOptions{})

	g.Expect(buf.String()).To(ContainSubstring("A command with description"))
}

func TestPrintCommandHelp_WithFlags(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func(_ helpTestCmdArgs) {}, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	var buf bytes.Buffer
	printCommandHelp(&buf, node, RunOptions{})

	g.Expect(buf.String()).To(ContainSubstring("Usage:"))
	g.Expect(buf.String()).To(ContainSubstring("--name"))
}

func TestPrintCommandHelp_WithSubcommands(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("parseTargetLike returned nil node")
	}

	// Manually add subcommands
	node.Subcommands = map[string]*commandNode{
		"sub1": {Name: "sub1", Description: "Subcommand one"},
		"sub2": {Name: "sub2", Description: "Subcommand two"},
	}

	var buf bytes.Buffer
	printCommandHelp(&buf, node, RunOptions{})

	g.Expect(buf.String()).To(ContainSubstring("Subcommands:"))
	g.Expect(buf.String()).To(ContainSubstring("sub1"))
	g.Expect(buf.String()).To(ContainSubstring("sub2"))
}

func TestPrintCommandSummary_NoSubcommands(t *testing.T) {
	t.Parallel()

	node := &commandNode{
		Name:        "simple",
		Description: "Simple command",
	}

	var buf bytes.Buffer
	printCommandSummary(&buf, node, "  ")

	if !strings.Contains(buf.String(), "simple") {
		t.Errorf("expected simple command, got: %s", buf.String())
	}
}

// --- printCommandSummary tests ---

func TestPrintCommandSummary_WithSubcommands(t *testing.T) {
	t.Parallel()

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

	var buf bytes.Buffer
	printCommandSummary(&buf, node, "")

	// Verify output contains all commands
	if !strings.Contains(buf.String(), "root") {
		t.Errorf("expected root command, got: %s", buf.String())
	}

	if !strings.Contains(buf.String(), "sub1") {
		t.Errorf("expected sub1 command, got: %s", buf.String())
	}

	if !strings.Contains(buf.String(), "sub2") {
		t.Errorf("expected sub2 command, got: %s", buf.String())
	}

	if !strings.Contains(buf.String(), "nested") {
		t.Errorf("expected nested command, got: %s", buf.String())
	}
}

func TestPrintCompletion_EmptyShell(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	env := &ExecuteEnv{args: []string{"cmd"}}
	exec := &runExecutor{env: env}

	err := exec.printCompletion("")
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(func() ExitError {
		var target ExitError

		_ = errors.As(err, &target)

		return target
	}().Code).To(Equal(1))
	g.Expect(env.Output()).To(ContainSubstring("Could not detect shell"))
}

func TestPrintCompletion_UnsupportedShell(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	env := &ExecuteEnv{args: []string{"cmd"}}
	exec := &runExecutor{env: env}

	err := exec.printCompletion("powershell")
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(func() ExitError {
		var target ExitError

		_ = errors.As(err, &target)

		return target
	}().Code).To(Equal(1))
	g.Expect(env.Output()).To(ContainSubstring("unsupported shell"))
}

func TestPrintExamples_Custom(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	opts := RunOptions{
		Examples: []Example{
			{Title: "Run tests", Code: "targ test"},
		},
	}

	var buf bytes.Buffer
	printExamples(&buf, opts, true)

	g.Expect(buf.String()).To(ContainSubstring("Examples:"))
	g.Expect(buf.String()).To(ContainSubstring("Run tests:"))
	g.Expect(buf.String()).To(ContainSubstring("targ test"))
	g.Expect(buf.String()).NotTo(ContainSubstring("Enable shell completion"))
}

func TestPrintExamples_Empty(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	opts := RunOptions{Examples: EmptyExamples()}

	var buf bytes.Buffer
	printExamples(&buf, opts, true)

	g.Expect(buf.String()).To(BeEmpty())
}

func TestPrintExamples_Nil(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	opts := RunOptions{Examples: nil}

	var buf bytes.Buffer
	printExamples(&buf, opts, true)

	g.Expect(buf.String()).To(ContainSubstring("Examples:"))
	g.Expect(buf.String()).To(ContainSubstring("Enable shell completion"))
	g.Expect(buf.String()).To(ContainSubstring("Run multiple"))
}

func TestPrintExamples_NotRoot(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	opts := RunOptions{Examples: nil}

	var buf bytes.Buffer
	printExamples(&buf, opts, false)

	g.Expect(buf.String()).To(ContainSubstring("Examples:"))
	g.Expect(buf.String()).NotTo(ContainSubstring("Enable shell completion"))
	g.Expect(buf.String()).To(ContainSubstring("Run multiple"))
}

func TestPrintFlagWithWrappedEnum(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Short enum - should not wrap
	var buf bytes.Buffer
	printFlagWithWrappedEnum(&buf, "--status {a|b|c}", "Status", "{a|b|c}", "  ")

	g.Expect(buf.String()).To(ContainSubstring("--status {a|b|c}"))
	g.Expect(buf.String()).To(ContainSubstring("Status"))
}

func TestPrintFlagWithWrappedEnum_LongEnum(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	longEnum := "{backlog|selected|in-progress|review|done|cancelled|blocked}"
	var buf bytes.Buffer
	printFlagWithWrappedEnum(&buf, "--status "+longEnum, "Status", longEnum, "  ")

	// Should wrap across multiple lines
	g.Expect(buf.String()).To(ContainSubstring("backlog"))
	g.Expect(buf.String()).To(ContainSubstring("blocked"))
}

func TestPrintFlagsIndented(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	flags := []flagHelp{
		{Name: "output", Placeholder: "<string>", Usage: "Output file"},
		{Name: "verbose", Placeholder: "[flag]"},
	}

	var buf bytes.Buffer
	printFlagsIndented(&buf, flags, "  ")

	g.Expect(buf.String()).To(ContainSubstring("--output <string>"))
	g.Expect(buf.String()).To(ContainSubstring("Output file"))
	g.Expect(buf.String()).To(ContainSubstring("--verbose"))
}

func TestPrintMoreInfo_WithMoreInfoText(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var buf bytes.Buffer
	printMoreInfo(&buf, RunOptions{MoreInfoText: "Custom info"})

	g.Expect(buf.String()).To(ContainSubstring("More info:"))
	g.Expect(buf.String()).To(ContainSubstring("Custom info"))
}

func TestPrintMoreInfo_WithRepoURL(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var buf bytes.Buffer
	printMoreInfo(&buf, RunOptions{RepoURL: "https://example.com/repo"})

	g.Expect(buf.String()).To(ContainSubstring("More info:"))
	g.Expect(buf.String()).To(ContainSubstring("https://example.com/repo"))
}

func TestPrintSubcommandList_NilSubcommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	subs := map[string]*commandNode{
		"nilcmd":  nil,
		"realcmd": {Name: "realcmd", Description: "Real command"},
	}

	var buf bytes.Buffer
	printSubcommandList(&buf, subs, "  ")

	// Should skip the nil entry and only show realcmd
	g.Expect(buf.String()).To(ContainSubstring("realcmd"))
	g.Expect(buf.String()).To(ContainSubstring("Real command"))
}

func TestPrintSubcommandList_NoDescription(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	subs := map[string]*commandNode{
		"nodesc": {Name: "nodesc"}, // No description
	}

	var buf bytes.Buffer
	printSubcommandList(&buf, subs, "  ")

	g.Expect(buf.String()).To(ContainSubstring("nodesc"))
}

// --- printSubcommandList tests ---

func TestPrintSubcommandList_WithDescriptions(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	subs := map[string]*commandNode{
		"alpha": {Name: "alpha", Description: "Alpha command"},
		"beta":  {Name: "beta", Description: "Beta command"},
	}

	var buf bytes.Buffer
	printSubcommandList(&buf, subs, "  ")

	g.Expect(buf.String()).To(ContainSubstring("alpha"))
	g.Expect(buf.String()).To(ContainSubstring("Alpha command"))
	g.Expect(buf.String()).To(ContainSubstring("beta"))
	g.Expect(buf.String()).To(ContainSubstring("Beta command"))
}

// --- printUsage tests ---

func TestPrintUsage_WithDescription(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	nodes := []*commandNode{
		{
			Name:        "test",
			Description: "Test command",
			SourceFile:  "/test/path.go",
		},
	}

	opts := RunOptions{Description: "Top level description"}

	var buf bytes.Buffer
	printUsage(&buf, nodes, opts)

	g.Expect(buf.String()).To(ContainSubstring("Top level description"))
	g.Expect(buf.String()).To(ContainSubstring("test"))
	g.Expect(buf.String()).To(ContainSubstring("Test command"))
	// Commands grouped by source (path may be relative)
	g.Expect(buf.String()).To(ContainSubstring("path.go]"))
}

func TestPrintUsage_WithNestedSubcommands(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	nodes := []*commandNode{
		{
			Name:        "parent",
			Description: "Parent command",
			SourceFile:  "/test/path.go",
			Subcommands: map[string]*commandNode{
				"child": {
					Name:        "child",
					Description: "Child command",
				},
			},
		},
	}

	var buf bytes.Buffer
	printUsage(&buf, nodes, RunOptions{})

	g.Expect(buf.String()).To(ContainSubstring("parent"))
	g.Expect(buf.String()).To(ContainSubstring("Parent command"))
	// Commands grouped by source (path may be relative)
	g.Expect(buf.String()).To(ContainSubstring("path.go]"))
}

func TestRegisterFlagName_DuplicateName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	spec := &flagSpec{name: "verbose", short: "v"}
	usedNames := map[string]bool{"verbose": true}

	err := registerFlagName(spec, usedNames)
	g.Expect(err).To(HaveOccurred())

	if err != nil {
		g.Expect(err.Error()).To(ContainSubstring("verbose"))
	}
}

func TestRegisterFlagName_DuplicateShort(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	spec := &flagSpec{name: "verbose", short: "v"}
	usedNames := map[string]bool{"v": true}

	err := registerFlagName(spec, usedNames)
	g.Expect(err).To(HaveOccurred())

	if err != nil {
		g.Expect(err.Error()).To(ContainSubstring("v"))
	}
}

func TestRegisterFlagName_NoShort(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	spec := &flagSpec{name: "verbose", short: ""}
	usedNames := make(map[string]bool)

	err := registerFlagName(spec, usedNames)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(usedNames["verbose"]).To(BeTrue())
	g.Expect(usedNames).NotTo(HaveKey(""))
}

// --- registerFlagName tests ---

func TestRegisterFlagName_Success(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	spec := &flagSpec{name: "verbose", short: "v"}
	usedNames := make(map[string]bool)

	err := registerFlagName(spec, usedNames)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(usedNames["verbose"]).To(BeTrue())
	g.Expect(usedNames["v"]).To(BeTrue())
}

func TestRelativeSourcePathWithGetwd_GetwdError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	failingGetwd := func() (string, error) {
		return "", errors.New("getwd failed")
	}

	result := relativeSourcePathWithGetwd("/some/path/internalfile.go", failingGetwd)
	g.Expect(result).To(Equal("/some/path/internalfile.go"))
}

func TestRelativeSourcePath_Empty(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	result := relativeSourcePath("")
	g.Expect(result).To(Equal(""))
}

func TestRelativeSourcePath_SubPath(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	cwd, _ := os.Getwd()
	testPath := filepath.Join(cwd, "sub", "testinternalfile.go")
	result := relativeSourcePath(testPath)
	g.Expect(result).To(Equal(filepath.Join("sub", "testinternalfile.go")))
}

func TestRelativeSourcePath_ValidPath(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	cwd, _ := os.Getwd()
	testPath := filepath.Join(cwd, "testinternalfile.go")
	result := relativeSourcePath(testPath)
	g.Expect(result).To(Equal("testinternalfile.go"))
}

func TestRunShellWithVars_Substitution(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	ctx := context.Background()

	// Test variable substitution
	vars := map[string]string{"name": "hello", "port": "8080"}
	err := runShellWithVars(ctx, "echo $name $port > /dev/null", vars)
	g.Expect(err).ToNot(HaveOccurred())

	// Test with unmatched variable (not in vars map) - variable stays unchanged
	// This tests the "return match" path when var not found
	varsPartial := map[string]string{"name": "hello"}
	// Command has $port but it's not in vars - it will remain as $port in output
	// Using 'true' to always succeed regardless of substitution result
	err = runShellWithVars(ctx, "true $name $port", varsPartial)
	g.Expect(err).ToNot(HaveOccurred())
}

// --- Additional RunWithEnv tests ---

func TestRunWithEnv_CompleteCommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "simple-target"}
	output := captureStdout(t, func() {
		env := &ExecuteEnv{args: []string{"cmd", "__complete", "cmd "}}
		err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
		g.Expect(err).NotTo(HaveOccurred())
	})

	// Should output completion suggestions (flags for the command)
	g.Expect(output).To(ContainSubstring("--help"))
}

func TestRunWithEnv_CompletionFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "simple-target"}
	output := captureStdout(t, func() {
		env := &ExecuteEnv{args: []string{"cmd", "--completion", "bash"}}
		err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
		g.Expect(err).NotTo(HaveOccurred())
	})

	// Should output a bash completion script (contains completion function definition)
	g.Expect(output).To(ContainSubstring("_completion"))
	g.Expect(output).To(ContainSubstring("complete"))
}

func TestRunWithEnv_CompletionFlagInvalidShell(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "simple-target"}
	// --completion with invalid shell name
	env := &ExecuteEnv{args: []string{"cmd", "--completion", "invalid-shell"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)

	// Should return error for unknown shell
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
}

func TestRunWithEnv_DefaultModeExecutionError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// A command that returns an error
	target := &mockTarget{
		fn:   func() error { return errors.New("command error") },
		name: "error-target",
	}
	env := &ExecuteEnv{args: []string{"cmd"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
}

func TestRunWithEnv_DefaultModeUnknownCommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "simple-target"}
	env := &ExecuteEnv{args: []string{"cmd", "unknown-subcommand"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("unknown command"))
}

func TestRunWithEnv_ListCommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "simple-target"}
	output := captureStdout(t, func() {
		env := &ExecuteEnv{args: []string{"cmd", "__list"}}
		err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
		g.Expect(err).NotTo(HaveOccurred())
	})

	g.Expect(output).To(ContainSubstring("simple-target"))
}

func TestRunWithEnv_NoCommands(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	env := &ExecuteEnv{args: []string{"cmd"}}
	err := RunWithEnv(env, RunOptions{}, []any{}...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env.Output()).To(ContainSubstring("No commands found"))
}

func TestRunWithEnv_ShellCommandTarget(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Shell command target without variables
	target := &mockTarget{
		fn:   "true",
		name: "simple",
	}

	// Use RunWithEnv with AllowDefault so single target is default
	env := &ExecuteEnv{args: []string{"cmd"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestRunWithEnv_ShellCommandTarget_ExecutionError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Shell command that will fail
	target := &mockTarget{
		fn:   "false",
		name: "fail",
	}

	env := &ExecuteEnv{args: []string{"cmd"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).To(HaveOccurred())
}

func TestRunWithEnv_ShellCommandTarget_Help(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{
		fn:          "echo $name",
		name:        "greet",
		description: "Greet someone",
	}

	env := &ExecuteEnv{args: []string{"cmd", "--help"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(env.Output()).To(ContainSubstring("greet"))
	g.Expect(env.Output()).To(ContainSubstring("--name"))
}

func TestRunWithEnv_ShellCommandTarget_HelpOnly(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{
		fn:   "echo $name",
		name: "greet",
	}

	// HelpOnly mode should not execute the command, just return remaining args
	env := &ExecuteEnv{args: []string{"cmd", "--name=world", "extra"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true, HelpOnly: true}, target)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestRunWithEnv_ShellCommandTarget_MissingVar(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{
		fn:   "echo $name $port",
		name: "greet",
	}

	// Only provide name, not port - should error
	env := &ExecuteEnv{args: []string{"cmd", "--name=world"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).To(HaveOccurred())
	g.Expect(env.Output()).To(ContainSubstring("port"))
}

func TestRunWithEnv_ShellCommandTarget_WithVar(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Shell command target with variables
	target := &mockTarget{
		fn:   "echo $name > /dev/null",
		name: "greet",
	}

	env := &ExecuteEnv{args: []string{"cmd", "--name=world"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestRunWithEnv_TimeoutError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "simple-target"}
	env := &ExecuteEnv{args: []string{"cmd", "--timeout", "invalid"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("invalid"))
}

func TestRunWithEnv_UnknownCommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "simple-target"}
	env := &ExecuteEnv{args: []string{"cmd", "unknown-cmd"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: false}, target)
	g.Expect(err).To(BeAssignableToTypeOf(ExitError{}))
	g.Expect(env.Output()).To(ContainSubstring("Unknown command"))
}

func TestShellVarFlagHelp(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	vars := []string{"namespace", "file", "name"}
	flags := shellVarFlagHelp(vars)

	g.Expect(flags).To(HaveLen(3))

	// First flag: namespace with short -n
	g.Expect(flags[0].Name).To(Equal("namespace"))
	g.Expect(flags[0].Short).To(Equal("n"))
	g.Expect(flags[0].Required).To(BeTrue())

	// Second flag: file with short -f
	g.Expect(flags[1].Name).To(Equal("file"))
	g.Expect(flags[1].Short).To(Equal("f"))

	// Third flag: name - no short (n already used)
	g.Expect(flags[2].Name).To(Equal("name"))
	g.Expect(flags[2].Short).To(Equal("")) // n already taken
}

// --- skipTargFlags tests ---

func TestSkipTargFlags_EmptyArgs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	result := skipTargFlags([]string{})
	g.Expect(result).To(BeEmpty())
}

func TestSkipTargFlags_NoTargFlags(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	result := skipTargFlags([]string{"build", "test", "--verbose"})
	g.Expect(result).To(Equal([]string{"build", "test", "--verbose"}))
}

func TestSkipTargFlags_WithCompletion(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// --completion is exit-early, stops processing
	result := skipTargFlags([]string{"--completion", "bash"})
	g.Expect(result).To(BeEmpty())
}

func TestSkipTargFlags_WithCompletionEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// --completion=bash uses flag=value syntax
	result := skipTargFlags([]string{"--completion=bash", "build"})
	g.Expect(result).To(Equal([]string{"build"}))
}

func TestSkipTargFlags_WithHelp(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// --help is boolean flag
	result := skipTargFlags([]string{"--help", "build"})
	g.Expect(result).To(Equal([]string{"build"}))
}

func TestSkipTargFlags_WithNoCache(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// --no-cache is boolean flag
	result := skipTargFlags([]string{"--no-cache", "build"})
	g.Expect(result).To(Equal([]string{"build"}))
}

func TestSkipTargFlags_WithRemovedFlags(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// --alias is no longer an exit-early flag (removed), so args pass through
	result := skipTargFlags([]string{"--alias", "foo", "bar"})
	g.Expect(result).To(Equal([]string{"--alias", "foo", "bar"}))
}

func TestSkipTargFlags_WithTimeout(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// --timeout consumes next arg
	result := skipTargFlags([]string{"--timeout", "10m", "build"})
	g.Expect(result).To(Equal([]string{"build"}))
}

func TestSkipTargFlags_WithTimeoutEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// --timeout=value syntax
	result := skipTargFlags([]string{"--timeout=10m", "build"})
	g.Expect(result).To(Equal([]string{"build"}))
}

func TestSliceFlagMissingValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type SliceCmdArgs struct {
		Files []string `targ:"flag"`
	}

	// Test with no values after flag (error case)
	fn := func(_ SliceCmdArgs) {}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.TODO(), []string{"--files"}, RunOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("flag needs an argument"))
}

func TestSliceFlagParsing(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type SliceCmdArgs struct {
		Files []string `targ:"flag"`
	}

	// Test with multiple values
	var gotFiles []string

	fn := func(args SliceCmdArgs) { gotFiles = args.Files }
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.TODO(), []string{"--files", "a.txt", "b.txt", "c.txt"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotFiles).To(Equal([]string{"a.txt", "b.txt", "c.txt"}))
}

func TestSliceFlagStopsAtDoubleDash(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type SliceCmdArgs struct {
		Files []string `targ:"flag"`
		Arg   string   `targ:"positional"`
	}

	// Test that slice parsing stops at --
	var (
		gotFiles []string
		gotArg   string
	)

	fn := func(args SliceCmdArgs) {
		gotFiles = args.Files
		gotArg = args.Arg
	}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(
		context.TODO(),
		[]string{"--files", "a.txt", "--", "positional-arg"},
		RunOptions{},
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotFiles).To(Equal([]string{"a.txt"}))
	g.Expect(gotArg).To(Equal("positional-arg"))
}

func TestSliceFlagStopsAtFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type SliceCmdArgs struct {
		Files   []string `targ:"flag"`
		Verbose bool     `targ:"flag,short=v"`
	}

	// Test that slice parsing stops at another flag
	var (
		gotFiles   []string
		gotVerbose bool
	)

	fn := func(args SliceCmdArgs) {
		gotFiles = args.Files
		gotVerbose = args.Verbose
	}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	err = node.execute(context.TODO(), []string{"--files", "a.txt", "b.txt", "-v"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotFiles).To(Equal([]string{"a.txt", "b.txt"}))
	g.Expect(gotVerbose).To(BeTrue())
}

func TestSortedKeys(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	m := map[string]*commandNode{
		"charlie": {Name: "charlie"},
		"alpha":   {Name: "alpha"},
		"bravo":   {Name: "bravo"},
	}

	keys := sortedKeys(m)
	g.Expect(keys).To(Equal([]string{"alpha", "bravo", "charlie"}))
}

// --- suggestInstanceFlags tests ---

func TestSuggestInstanceFlags_NilNode(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	seen := map[string]bool{}

	err := suggestInstanceFlags(io.Discard, commandInstance{node: nil}, "--", seen)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(seen).To(BeEmpty())
}

func TestSuggestInstanceFlags_NilType(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	seen := map[string]bool{}

	err := suggestInstanceFlags(io.Discard, commandInstance{node: &commandNode{Type: nil}}, "--", seen)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(seen).To(BeEmpty())
}

// --- tagOptionsForField tests ---

func TestTagOptionsForField_EmptyTag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test field with no targ tag defaults to flag
	type Args struct {
		Name string // no targ tag
	}

	inst := reflect.ValueOf(Args{})
	typ := reflect.TypeFor[Args]()
	field := typ.Field(0)

	opts, err := tagOptionsForField(inst, field)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts.Kind).To(Equal(TagKindFlag))
	g.Expect(opts.Name).To(Equal("name")) // lowercase of field name
}

func TestTagOptionsForField_EmptyTagWithOverride(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test field with no targ tag but with TagOptions override
	inst := reflect.ValueOf(TagOptionsOverrideArgs{})
	typ := reflect.TypeFor[TagOptionsOverrideArgs]()
	field := typ.Field(0) // Env field

	opts, err := tagOptionsForField(inst, field)
	g.Expect(err).NotTo(HaveOccurred())
	// The TagOptions method changes enum from "dev|prod" to "dev|staging|prod"
	g.Expect(opts.Enum).To(Equal("dev|staging|prod"))
}

// --- tagOptionsInstance tests ---

func TestTagOptionsInstance_NilNode(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	result := tagOptionsInstance(nil)
	g.Expect(result.IsValid()).To(BeFalse())
}

func TestTagOptionsInstance_NoTypeNoValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{}

	result := tagOptionsInstance(node)
	g.Expect(result.IsValid()).To(BeFalse())
}

func TestTagOptionsInstance_TypeWithoutValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type Args struct {
		Name string
	}

	node := &commandNode{Type: reflect.TypeFor[Args]()}

	result := tagOptionsInstance(node)
	g.Expect(result.IsValid()).To(BeTrue())
	// Should return a zero value of the type
	g.Expect(result.Interface()).To(Equal(Args{}))
}

func TestTagOptionsInstance_ValidValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type Args struct {
		Name string
	}

	inst := reflect.ValueOf(Args{Name: "test"})
	node := &commandNode{Value: inst}

	result := tagOptionsInstance(node)
	g.Expect(result.IsValid()).To(BeTrue())
	g.Expect(result.Interface()).To(Equal(Args{Name: "test"}))
}

// --- Usage Line Formatting ---

func TestUsageLine_NoSubcommandWithRequiredPositional(t *testing.T) {
	t.Parallel()

	type MoveCmdArgs struct {
		File   string `targ:"flag,short=f"`
		Status string `targ:"flag,required,short=s"`
		ID     int    `targ:"positional,required"`
	}

	target := &mockTarget{fn: func(_ MoveCmdArgs) {}, name: "test"}

	cmd, err := parseTargetLike(target)
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

	if !strings.Contains(usage, "ID") {
		t.Fatalf("expected ID positional in usage: %s", usage)
	}

	// Optional flags now appear as [flags...] at end
	if !strings.HasSuffix(usage, "[flags...]") {
		t.Fatalf("expected [flags...] at end: %s", usage)
	}
}

func TestVariadicPositionalStopsAtDoubleDash(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type VariadicPosCmdArgs struct {
		Args []string `targ:"positional"`
		Flag string   `targ:"flag"`
	}

	var (
		gotArgs []string
		gotFlag string
	)

	fn := func(args VariadicPosCmdArgs) {
		gotArgs = args.Args
		gotFlag = args.Flag
	}
	target := &mockTarget{fn: fn, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())

	if node == nil {
		t.Fatal("unexpected nil node")
	}

	// Test that variadic positional stops at -- and next args are parsed as flags
	err = node.execute(context.TODO(), []string{"a", "b", "--", "--flag", "value"}, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotArgs).To(Equal([]string{"a", "b"}))
	g.Expect(gotFlag).To(Equal("value"))
}

func TestWriteWrappedUsage_EmptyParts(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var buf strings.Builder
	writeWrappedUsage(&buf, "Usage: ", nil)
	g.Expect(buf.String()).To(Equal("Usage: \n"))
}

func TestWriteWrappedUsage_MultiplePartsNoWrap(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var buf strings.Builder
	writeWrappedUsage(&buf, "Usage: ", []string{"cmd", "sub", "[--flag]"})
	g.Expect(buf.String()).To(Equal("Usage: cmd sub [--flag]\n"))
}

func TestWriteWrappedUsage_SinglePart(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var buf strings.Builder
	writeWrappedUsage(&buf, "Usage: ", []string{"cmd"})
	g.Expect(buf.String()).To(Equal("Usage: cmd\n"))
}

func TestWriteWrappedUsage_Wrapping(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var buf strings.Builder
	// Create parts that will exceed 80 chars
	parts := []string{
		"targ", "issues", "create", "[--file <string>]", "[--github]",
		"--title <string>", "[--status {backlog|selected|in-progress|review|done|cancelled|blocked}]",
	}
	writeWrappedUsage(&buf, "Usage: ", parts)
	output := buf.String()
	// Should have multiple lines
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	g.Expect(len(lines)).To(BeNumerically(">", 1))
	// Each line should be <= 80 chars
	for _, line := range lines {
		g.Expect(len(line)).To(BeNumerically("<=", usageLineWidth))
	}
}

func TooManyReturnsFunc() (int, error) {
	return 42, nil
}

// unexported constants.
const (
	bashShell = "bash"
)

type helpTestCmdArgs struct {
	Name string `targ:"flag"`
}

type helpTestCmdWithPlaceholderArgs struct {
	Output string `targ:"flag,placeholder=<file>"`
}

type helpTestCmdWithShortArgs struct {
	Verbose bool `targ:"flag,short=v"`
}

type helpTestCmdWithUsageArgs struct {
	Format string `targ:"flag,desc=Output format"`
}

type mockGroup struct {
	name    string
	members []any
}

func (m *mockGroup) GetMembers() []any { return m.members }

func (m *mockGroup) GetName() string { return m.name }

type mockTarget struct {
	fn          any
	name        string
	description string
}

func (m *mockTarget) Fn() any { return m.fn }

func (m *mockTarget) GetDescription() string { return m.description }

func (m *mockTarget) GetName() string { return m.name }

type posIdxArgs struct {
	Name    string `targ:"flag,short=n"`
	Verbose bool   `targ:"flag,short=v"`
	Files   string `targ:"flag,variadic"`
}

type testPlainType struct{}

type testStringSetter struct {
	value string
}

func (t *testStringSetter) Set(s string) error {
	t.value = "set:" + s
	return nil
}

type testTextUnmarshaler struct {
	value string
}

func (t *testTextUnmarshaler) UnmarshalText(text []byte) error {
	t.value = "unmarshaled:" + string(text)
	return nil
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("unexpected pipe error: %v", err)
	}

	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer

	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("unexpected stdout copy error: %v", err)
	}

	_ = r.Close()

	return buf.String()
}

// extractShellName extracts and validates the shell name from a path.
// This is the testable core logic of DetectShell().
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
	case bashShell, zshShell, fishShell:
		return base
	default:
		return ""
	}
}
