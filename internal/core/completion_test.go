package core

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

type CompletionDiscoverRoot struct{}

func (c *CompletionDiscoverRoot) Name() string { return "discover" }

func (c *CompletionDiscoverRoot) Run() {}

type CompletionFirmwareRoot struct {
	FlashOnly *CompletionFlashOnly `targ:"subcommand=flash-only"`
}

func (c *CompletionFirmwareRoot) Name() string { return "firmware" }

type CompletionFlashOnly struct{}

func (c *CompletionFlashOnly) Run() {}

type EnumCmd struct {
	Mode string `targ:"flag,enum=dev|prod,short=m"`
	Kind string `targ:"flag,enum=fast|slow"`
}

func (c *EnumCmd) Run() {}

type EnumOverrideCmd struct {
	Mode string `targ:"flag,enum=dev|prod"`
}

func (c *EnumOverrideCmd) Run() {}

func (c *EnumOverrideCmd) TagOptions(field string, opts TagOptions) (TagOptions, error) {
	if field == "Mode" {
		opts.Enum = "alpha|beta"
	}

	return opts, nil
}

type PositionalCompletionCmd struct {
	Status string `targ:"flag"`
	ID     int    `targ:"positional"`
}

func (c *PositionalCompletionCmd) Run() {}

func (c *PositionalCompletionCmd) TagOptions(field string, opts TagOptions) (TagOptions, error) {
	if field != "ID" {
		return opts, nil
	}

	if c.Status == "cancelled" {
		opts.Enum = "40|41"
	}

	return opts, nil
}

// VariadicFlagCmd has a slice flag to test variadic value skipping.
type VariadicFlagCmd struct {
	Files  []string `targ:"flag"`
	Target string   `targ:"positional,enum=build|test"`
}

func (c *VariadicFlagCmd) Run() {}

func TestCompletionChain_NilNode(t *testing.T) {
	chain, err := completionChain(nil, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chain != nil {
		t.Fatalf("expected nil chain, got %v", chain)
	}
}

func TestCompletionSuggestsEnumValuesAfterFlag(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{cmd}, "app --mode ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "dev") || !strings.Contains(out, "prod") {
		t.Fatalf("expected enum suggestions, got: %q", out)
	}
}

func TestCompletionSuggestsPositionalValues(t *testing.T) {
	cmd, err := parseCommand(&PositionalCompletionCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{cmd}, "app --status cancelled ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "40") || !strings.Contains(out, "41") {
		t.Fatalf("expected positional suggestions, got: %q", out)
	}
}

func TestCompletionSuggestsRootsAfterCommand(t *testing.T) {
	firmware, err := parseCommand(&CompletionFirmwareRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	discover, err := parseCommand(&CompletionDiscoverRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{firmware, discover}, "app firmware flash-only d")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "discover") {
		t.Fatalf("expected discover suggestion, got: %q", out)
	}
}

func TestDoCompletion_CaretSuggestion(t *testing.T) {
	firmware, err := parseCommand(&CompletionFirmwareRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	discover, err := parseCommand(&CompletionDiscoverRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{firmware, discover}, "app firmware flash-only ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "^") {
		t.Fatalf("expected ^ suggestion, got: %q", out)
	}
}

func TestDoCompletion_ChainedRootCommands(t *testing.T) {
	firmware, err := parseCommand(&CompletionFirmwareRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	discover, err := parseCommand(&CompletionDiscoverRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After running firmware, discover is a remaining arg that should chain
	// "app firmware discover " - after discover completes, suggests roots again
	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{firmware, discover}, "app firmware discover ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	// After chaining through both commands, should suggest roots again
	if !strings.Contains(out, "firmware") || !strings.Contains(out, "discover") {
		t.Fatalf("expected root suggestions after chained commands, got: %q", out)
	}
}

// --- doCompletion additional tests ---

func TestDoCompletion_EmptyCommandLine(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty command line after tokenization
	err = doCompletion([]*commandNode{cmd}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoCompletion_FlagSuggestion(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{cmd}, "app --")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "--mode") {
		t.Fatalf("expected --mode flag suggestion, got: %q", out)
	}
}

func TestDoCompletion_MultipleRootsAtRootLevel(t *testing.T) {
	firmware, err := parseCommand(&CompletionFirmwareRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	discover, err := parseCommand(&CompletionDiscoverRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{firmware, discover}, "app ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "firmware") || !strings.Contains(out, "discover") {
		t.Fatalf("expected root suggestions, got: %q", out)
	}
}

func TestDoCompletion_MultipleRootsWithPrefix(t *testing.T) {
	firmware, err := parseCommand(&CompletionFirmwareRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	discover, err := parseCommand(&CompletionDiscoverRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{firmware, discover}, "app f")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "firmware") {
		t.Fatalf("expected firmware suggestion, got: %q", out)
	}
}

func TestDoCompletion_PartialRootMatchSuggestsMatchingRoots(t *testing.T) {
	firmware, err := parseCommand(&CompletionFirmwareRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	discover, err := parseCommand(&CompletionDiscoverRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 'firm ' (with trailing space) doesn't exactly match any root,
	// but should suggest roots starting with 'firm'
	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{firmware, discover}, "app firm ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "firmware") {
		t.Fatalf("expected firmware suggestion for partial match, got: %q", out)
	}
}

func TestDoCompletion_SingleRootAtRoot(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{cmd}, "app ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	// Single root at root level should suggest the root command name
	if !strings.Contains(out, "enum-cmd") {
		t.Fatalf("expected enum-cmd suggestion, got: %q", out)
	}
}

func TestDoCompletion_UnknownRootPrefix(t *testing.T) {
	firmware, err := parseCommand(&CompletionFirmwareRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	discover, err := parseCommand(&CompletionDiscoverRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 'xyz' doesn't match any root
	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{firmware, discover}, "app xyz")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	// Should not suggest anything since no match
	if out != "" {
		t.Fatalf("expected no suggestions for unknown prefix, got: %q", out)
	}
}

func TestDoCompletion_VariadicFlagSkipsMultipleValues(t *testing.T) {
	cmd, err := parseCommand(&VariadicFlagCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// --files is variadic (slice), values a.txt b.txt should be skipped
	// Then we should get positional enum suggestions
	out := captureStdout(t, func() {
		err := doCompletion([]*commandNode{cmd}, "app --files a.txt b.txt ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Should suggest positional enum values after skipping variadic flag values
	if !strings.Contains(out, "build") || !strings.Contains(out, "test") {
		t.Fatalf("expected positional enum suggestions after variadic flag values, got: %q", out)
	}
}

func TestEnumValuesForArg_LongFlag(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chain, err := completionChain(cmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	values, ok, err := enumValuesForArg(chain, []string{"--mode"}, "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ok {
		t.Fatal("expected enum values for --mode")
	}

	if len(values) != 2 || values[0] != "dev" || values[1] != "prod" {
		t.Fatalf("unexpected values: %v", values)
	}
}

func TestEnumValuesForArg_NoMatch(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chain, err := completionChain(cmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if values, ok, err := enumValuesForArg(chain, []string{"--unknown"}, "", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if ok {
		t.Fatalf("expected no enum values, got %v", values)
	}
}

func TestEnumValuesForArg_ShortFlag(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chain, err := completionChain(cmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	values, ok, err := enumValuesForArg(chain, []string{"-m"}, "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ok {
		t.Fatal("expected enum values for -m")
	}

	if len(values) != 2 || values[0] != "dev" || values[1] != "prod" {
		t.Fatalf("unexpected values: %v", values)
	}
}

func TestEnumValuesForArg_TagOptionsOverride(t *testing.T) {
	cmd, err := parseCommand(&EnumOverrideCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chain, err := completionChain(cmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	values, ok, err := enumValuesForArg(chain, []string{"--mode"}, "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ok {
		t.Fatal("expected enum values for --mode")
	}

	if len(values) != 2 || values[0] != "alpha" || values[1] != "beta" {
		t.Fatalf("unexpected values: %v", values)
	}
}

func TestFindCompletionRoot_CaseInsensitive(t *testing.T) {
	roots := []*commandNode{
		{Name: "Build"},
	}

	result := findCompletionRoot(roots, "build")
	if result == nil {
		t.Fatal("expected case-insensitive match")
	}
}

func TestFindCompletionRoot_Found(t *testing.T) {
	roots := []*commandNode{
		{Name: "build"},
		{Name: "test"},
		{Name: "run"},
	}

	result := findCompletionRoot(roots, "test")
	if result == nil {
		t.Fatal("expected to find 'test' root")
	}

	if result.Name != "test" {
		t.Fatalf("expected name 'test', got %q", result.Name)
	}
}

func TestFindCompletionRoot_NotFound(t *testing.T) {
	roots := []*commandNode{
		{Name: "build"},
	}

	result := findCompletionRoot(roots, "unknown")
	if result != nil {
		t.Fatal("expected nil for unknown root")
	}
}

func TestFollowRemaining_MultiRoot_Found(t *testing.T) {
	root1, err := parseStruct(&CompletionFirmwareRoot{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	root2, err := parseStruct(&CompletionDiscoverRoot{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	state := &completionState{
		roots:       []*commandNode{root1, root2},
		singleRoot:  false,
		currentNode: root1,
	}

	result := parseResult{
		remaining: []string{"discover", "arg1"},
	}

	ok := state.followRemaining(result)
	if !ok {
		t.Fatal("expected followRemaining to return true when root found")
	}

	if state.currentNode != root2 {
		t.Fatal("expected currentNode to be switched to discovered root")
	}

	if !state.explicit {
		t.Fatal("expected explicit to be true for multi root")
	}
}

func TestFollowRemaining_MultiRoot_NotFound(t *testing.T) {
	root1, err := parseStruct(&CompletionFirmwareRoot{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	state := &completionState{
		roots:       []*commandNode{root1},
		singleRoot:  false,
		currentNode: root1,
	}

	result := parseResult{
		remaining: []string{"unknown-cmd", "arg1"},
	}

	ok := state.followRemaining(result)
	if ok {
		t.Fatal("expected followRemaining to return false when root not found")
	}
}

func TestFollowRemaining_SingleRoot(t *testing.T) {
	root, err := parseStruct(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	state := &completionState{
		roots:      []*commandNode{root},
		singleRoot: true,
	}

	result := parseResult{
		remaining: []string{"arg1", "arg2"},
	}

	ok := state.followRemaining(result)
	if !ok {
		t.Fatal("expected followRemaining to return true for single root")
	}

	if state.currentNode != root {
		t.Fatal("expected currentNode to be reset to first root")
	}

	if len(state.processedArgs) != 2 {
		t.Fatalf("expected processedArgs to be remaining, got %v", state.processedArgs)
	}

	if state.explicit {
		t.Fatal("expected explicit to be false for single root")
	}
}

func TestHasExitEarlyFlagPrefix_Match(t *testing.T) {
	if !hasExitEarlyFlagPrefix("--alias=something") {
		t.Fatal("expected true for --alias=something")
	}
}

func TestHasExitEarlyFlagPrefix_NoMatch(t *testing.T) {
	if hasExitEarlyFlagPrefix("--other") {
		t.Fatal("expected false for --other")
	}
}

func TestPrintCompletionScriptPlaceholders(t *testing.T) {
	cases := []string{"bash", "zsh", "fish"}
	for _, shell := range cases {
		out := captureStdout(t, func() {
			err := PrintCompletionScript(shell, "demo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if strings.Contains(out, "MISSING") {
			t.Fatalf("unexpected placeholder output for %s: %s", shell, out)
		}

		if !strings.Contains(out, "demo") {
			t.Fatalf("expected output to include binary name for %s", shell)
		}
	}
}

func TestSuggestPositionalEnumsOrRoots_ExpectingFlagValue(t *testing.T) {
	// When expecting a flag value, should return early
	root, err := parseStruct(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	state := &completionState{
		roots:         []*commandNode{root},
		currentNode:   root,
		processedArgs: []string{"--mode"}, // expecting flag value
		chain: []commandInstance{
			{node: root, value: root.Value},
		},
	}

	err = state.suggestPositionalEnumsOrRoots()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSuggestPositionalEnums_EmptyChain(t *testing.T) {
	// When chain is empty, should return false, nil
	root, err := parseStruct(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	state := &completionState{
		roots:       []*commandNode{root},
		currentNode: root,
		chain:       nil, // empty chain
	}

	suggested, err := state.suggestPositionalEnums()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if suggested {
		t.Fatal("expected suggested to be false for empty chain")
	}
}

func TestSuggestPositionalEnums_NoEnumField(t *testing.T) {
	// Command with no positional enums
	type NoEnumCmd struct {
		Name string `targ:"positional"`
	}

	root, err := parseStruct(&NoEnumCmd{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	state := &completionState{
		roots:         []*commandNode{root},
		currentNode:   root,
		processedArgs: []string{},
		chain: []commandInstance{
			{node: root, value: root.Value},
		},
	}

	suggested, err := state.suggestPositionalEnums()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if suggested {
		t.Fatal("expected suggested to be false for non-enum positional")
	}
}

func TestTokenizeCommandLine_BackslashInSingleQuotes(t *testing.T) {
	// In single quotes, backslash is literal
	parts, _ := tokenizeCommandLine(`'foo\bar'`)
	if len(parts) != 1 || parts[0] != `foo\bar` {
		t.Fatalf("unexpected parts: %v", parts)
	}
}

func TestTokenizeCommandLine_DoubleQuoteInsideSingle(t *testing.T) {
	parts, _ := tokenizeCommandLine(`'foo"bar'`)
	if len(parts) != 1 || parts[0] != `foo"bar` {
		t.Fatalf("unexpected parts: %v", parts)
	}
}

func TestTokenizeCommandLine_DoubleQuotes(t *testing.T) {
	parts, isNewArg := tokenizeCommandLine(`foo "bar baz"`)
	if len(parts) != 2 || parts[0] != "foo" || parts[1] != "bar baz" {
		t.Fatalf("unexpected parts: %v", parts)
	}

	if isNewArg {
		t.Fatal("expected isNewArg=false")
	}
}

func TestTokenizeCommandLine_EscapeInDoubleQuotes(t *testing.T) {
	parts, _ := tokenizeCommandLine(`"foo\"bar"`)
	if len(parts) != 1 || parts[0] != `foo"bar` {
		t.Fatalf("unexpected parts: %v", parts)
	}
}

func TestTokenizeCommandLine_EscapedSpace(t *testing.T) {
	parts, isNewArg := tokenizeCommandLine(`foo bar\ baz`)
	if len(parts) != 2 || parts[0] != "foo" || parts[1] != "bar baz" {
		t.Fatalf("unexpected parts: %v", parts)
	}

	if isNewArg {
		t.Fatal("expected isNewArg=false")
	}
}

func TestTokenizeCommandLine_NewlineSeparator(t *testing.T) {
	parts, isNewArg := tokenizeCommandLine("foo\nbar")
	if len(parts) != 2 || parts[0] != "foo" || parts[1] != "bar" {
		t.Fatalf("unexpected parts: %v", parts)
	}

	if isNewArg {
		t.Fatal("expected isNewArg=false")
	}
}

// --- tokenizeCommandLine tests ---

func TestTokenizeCommandLine_SimpleArgs(t *testing.T) {
	parts, isNewArg := tokenizeCommandLine("foo bar baz")
	if len(parts) != 3 || parts[0] != "foo" || parts[1] != "bar" || parts[2] != "baz" {
		t.Fatalf("unexpected parts: %v", parts)
	}

	if isNewArg {
		t.Fatal("expected isNewArg=false")
	}
}

func TestTokenizeCommandLine_SingleQuoteInsideDouble(t *testing.T) {
	parts, _ := tokenizeCommandLine(`"foo'bar"`)
	if len(parts) != 1 || parts[0] != "foo'bar" {
		t.Fatalf("unexpected parts: %v", parts)
	}
}

func TestTokenizeCommandLine_SingleQuotes(t *testing.T) {
	parts, isNewArg := tokenizeCommandLine("foo 'bar baz'")
	if len(parts) != 2 || parts[0] != "foo" || parts[1] != "bar baz" {
		t.Fatalf("unexpected parts: %v", parts)
	}

	if isNewArg {
		t.Fatal("expected isNewArg=false")
	}
}

func TestTokenizeCommandLine_TabSeparator(t *testing.T) {
	parts, isNewArg := tokenizeCommandLine("foo\tbar")
	if len(parts) != 2 || parts[0] != "foo" || parts[1] != "bar" {
		t.Fatalf("unexpected parts: %v", parts)
	}

	if isNewArg {
		t.Fatal("expected isNewArg=false")
	}
}

func TestTokenizeCommandLine_TrailingBackslash(t *testing.T) {
	parts, _ := tokenizeCommandLine(`foo\`)
	if len(parts) != 1 || parts[0] != `foo\` {
		t.Fatalf("unexpected parts: %v", parts)
	}
}

func TestTokenizeCommandLine_TrailingSpace(t *testing.T) {
	parts, isNewArg := tokenizeCommandLine("foo ")
	if len(parts) != 1 || parts[0] != "foo" {
		t.Fatalf("unexpected parts: %v", parts)
	}

	if !isNewArg {
		t.Fatal("expected isNewArg=true for trailing space")
	}
}

func TestTokenizeCommandLine_UnclosedDoubleQuote(t *testing.T) {
	parts, isNewArg := tokenizeCommandLine(`foo "bar`)
	if len(parts) != 2 || parts[0] != "foo" || parts[1] != "bar" {
		t.Fatalf("unexpected parts: %v", parts)
	}

	if isNewArg {
		t.Fatal("expected isNewArg=false for unclosed quote")
	}
}

func TestTokenizeCommandLine_UnclosedSingleQuote(t *testing.T) {
	parts, isNewArg := tokenizeCommandLine("foo 'bar")
	if len(parts) != 2 || parts[0] != "foo" || parts[1] != "bar" {
		t.Fatalf("unexpected parts: %v", parts)
	}

	if isNewArg {
		t.Fatal("expected isNewArg=false for unclosed quote")
	}
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
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("unexpected stdout copy error: %v", err)
	}

	_ = r.Close()

	return buf.String()
}
