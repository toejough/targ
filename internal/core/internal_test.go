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
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
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

// --- camelToKebab ---

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

// --- collectInstanceEnums tests ---

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

func TestCustomSetter_NonAddressable(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Create a non-addressable value
	val := reflect.ValueOf(testPlainType{})

	setter, ok := customSetter(val)
	g.Expect(ok).To(BeFalse())
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

// --- Git URL detection tests ---

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

// --- doList tests ---

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

// --- executeDefaultParallel tests ---

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

// --- ExitError tests ---

func TestExitError_Error(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	err := ExitError{Code: 42}
	g.Expect(err.Error()).To(Equal("exit code 42"))
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

// --- expectingFlagValue tests ---

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

func TestExpectingFlagValue_LongFlagNeedsValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"--name": {TakesValue: true},
	}
	g.Expect(expectingFlagValue([]string{"--name"}, specs)).To(BeTrue())
}

func TestExpectingFlagValue_LongFlagWithEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"--name": {TakesValue: true},
	}
	g.Expect(expectingFlagValue([]string{"--name=foo"}, specs)).To(BeFalse())
}

func TestExpectingFlagValue_ShortFlagNeedsValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	specs := map[string]completionFlagSpec{
		"-n": {TakesValue: true},
	}
	g.Expect(expectingFlagValue([]string{"-n"}, specs)).To(BeTrue())
}

// --- extractHelpFlag tests ---

// --- extractTimeout tests ---

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

func TestFuncSourceFile_InvalidValue(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	result := funcSourceFile(reflect.Value{})
	g.Expect(result).To(Equal(""))
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

func TestHandleHelpFlag_WithHelpFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	node := &commandNode{Name: "test", Description: "A test command"}

	var buf bytes.Buffer
	remaining, handled := handleHelpFlag(node, []string{"--help", "arg1"}, RunOptions{Stdout: &buf})
	g.Expect(handled).To(BeTrue())
	g.Expect(remaining).To(Equal([]string{"arg1"}))
	g.Expect(buf.String()).To(ContainSubstring("test"))
}

func TestHandleList_ReturnsErrorFromListFn(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	env := &ExecuteEnv{args: []string{"cmd", "__list"}}
	listErr := errors.New("list failed")

	exec := &runExecutor{
		env:    env,
		roots:  []*commandNode{{Name: "test"}},
		listFn: func(_ io.Writer, _ []*commandNode) error { return listErr },
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

// --- Glob pattern tests (whitebox) ---

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

// --- Parsing Edge Cases ---

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

func TestPrintCommandHelp_FlagWithUsage(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func(_ helpTestCmdWithUsageArgs) {}, name: "test"}
	node, err := parseTargetLike(target)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(node).NotTo(BeNil())

	var buf bytes.Buffer
	printCommandHelp(&buf, node, RunOptions{})

	g.Expect(buf.String()).To(ContainSubstring("Output format"))
}

// --- printCommandHelp tests ---

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

func TestPrintMoreInfo_WithMoreInfoText(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var buf bytes.Buffer
	printMoreInfo(&buf, RunOptions{MoreInfoText: "Custom info"})

	g.Expect(buf.String()).To(ContainSubstring("More info:"))
	g.Expect(buf.String()).To(ContainSubstring("Custom info"))
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

// --- registerFlagName tests ---

func TestRelativeSourcePathWithGetwd_GetwdError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	failingGetwd := func() (string, error) {
		return "", errors.New("getwd failed")
	}

	result := relativeSourcePathWithGetwd("/some/path/internalfile.go", failingGetwd)
	g.Expect(result).To(Equal("/some/path/internalfile.go"))
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
	env := &ExecuteEnv{args: []string{"cmd", "__complete", "cmd "}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).NotTo(HaveOccurred())

	// Should output completion suggestions (flags for the command)
	g.Expect(env.Output()).To(ContainSubstring("--help"))
}

func TestRunWithEnv_CompletionFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "simple-target"}
	env := &ExecuteEnv{args: []string{"cmd", "--completion", "bash"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).NotTo(HaveOccurred())

	// Should output a bash completion script (contains completion function definition)
	g.Expect(env.Output()).To(ContainSubstring("_completion"))
	g.Expect(env.Output()).To(ContainSubstring("complete"))
}

func TestRunWithEnv_ListCommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := &mockTarget{fn: func() {}, name: "simple-target"}
	env := &ExecuteEnv{args: []string{"cmd", "__list"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: true}, target)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(env.Output()).To(ContainSubstring("simple-target"))
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

// --- skipTargFlags tests ---

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

// --- suggestInstanceFlags tests ---

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

// --- tagOptionsInstance tests ---

func TestTagOptionsInstance_NilNode(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	result := tagOptionsInstance(nil)
	g.Expect(result.IsValid()).To(BeFalse())
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
