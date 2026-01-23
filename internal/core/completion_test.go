package core_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/toejough/targ/internal/core"
)

// --- Args struct types for Target functions ---

type EnumCmdArgs struct {
	Mode string `targ:"flag,enum=dev|prod,short=m"`
	Kind string `targ:"flag,enum=fast|slow"`
}

// EnumOverrideCmdArgs uses static enum (replacing dynamic TagOptions).
type EnumOverrideCmdArgs struct {
	Mode string `targ:"flag,enum=alpha|beta"`
}

// PositionalCompletionCmdArgs - simplified version without dynamic TagOptions.
// The dynamic enum based on Status is a struct-only feature.
type PositionalCompletionCmdArgs struct {
	Status string `targ:"flag"`
	ID     int    `targ:"positional,enum=10|20|30"`
}

type VariadicFlagCmdArgs struct {
	Files  []string `targ:"flag"`
	Target string   `targ:"positional,enum=build|test"`
}

func TestCompletion_BackslashInDoubleQuotes(t *testing.T) {
	// Test backslash escape inside double quotes
	target := core.Targ(func(_ EnumCmdArgs) {}).Name("enum-cmd")
	out := captureCompletion(t, target, `app --mode "de\"`)
	// The \" is an escaped quote, not end of string
	if strings.Contains(out, "dev") {
		t.Fatalf("expected no match with escaped quote, got: %q", out)
	}
}

func TestCompletion_CaretSuggestion(t *testing.T) {
	flashOnly := core.Targ(func() {}).Name("flash-only")
	firmware := core.NewGroup("firmware", flashOnly)
	discover := core.Targ(func() {}).Name("discover")

	out := captureCompletionMulti(t, []any{firmware, discover},
		"app firmware flash-only ")
	if !strings.Contains(out, "^") {
		t.Fatalf("expected ^ suggestion, got: %q", out)
	}
}

func TestCompletion_ChainedRootCommands(t *testing.T) {
	flashOnly := core.Targ(func() {}).Name("flash-only")
	firmware := core.NewGroup("firmware", flashOnly)
	discover := core.Targ(func() {}).Name("discover")

	out := captureCompletionMulti(t, []any{firmware, discover},
		"app firmware discover ")
	// After chaining through both commands, should suggest roots again
	if !strings.Contains(out, "firmware") || !strings.Contains(out, "discover") {
		t.Fatalf("expected root suggestions after chained commands, got: %q", out)
	}
}

func TestCompletion_EnumFlagFollowedByDash(t *testing.T) {
	// Test case where after an enum flag, user is typing another flag (prefix starts with -)
	// This should NOT suggest enum values since we're clearly typing a new flag
	target := core.Targ(func(_ EnumCmdArgs) {}).Name("enum-cmd")
	out := captureCompletion(t, target, "app --mode -")
	// Should NOT suggest dev/prod since prefix "-" indicates we're typing a flag
	// Instead should suggest flags that start with "-"
	if strings.Contains(out, "dev") || strings.Contains(out, "prod") {
		t.Fatalf("expected no enum values when prefix is -, got: %q", out)
	}
}

func TestCompletion_EnumFlagFollowedByNonEnumArg(t *testing.T) {
	// Test case where previous arg is not an enum flag (exercises final return nil)
	target := core.Targ(func(_ EnumCmdArgs) {}).Name("enum-cmd")
	out := captureCompletion(t, target, "app --mode dev notaflag ")
	// "notaflag" doesn't match any enum flag, so enumValuesForArg returns nil
	// In single-root mode, should suggest the root command
	if !strings.Contains(out, "enum-cmd") {
		t.Fatalf("expected root command suggestion after non-flag arg, got: %q", out)
	}
}

func TestCompletion_EscapedSpace(t *testing.T) {
	// Test escaped space in argument
	target := core.Targ(func(_ EnumCmdArgs) {}).Name("enum-cmd")
	out := captureCompletion(t, target, `app --mode de\ `)
	// The escaped space is part of the arg, so "de " doesn't match any enum
	if strings.Contains(out, "dev") {
		t.Fatalf("expected no dev suggestion with escaped space, got: %q", out)
	}
}

func TestCompletion_FlagSuggestion(t *testing.T) {
	target := core.Targ(func(_ EnumCmdArgs) {}).Name("enum-cmd")

	out := captureCompletion(t, target, "app --")
	if !strings.Contains(out, "--mode") {
		t.Fatalf("expected --mode flag suggestion, got: %q", out)
	}
}

func TestCompletion_MultiRootChainedRemaining(t *testing.T) {
	// Test multi-root mode where remaining args DO match a root
	// After firmware flash-only runs, "discover" matches a root so we chain to it
	flashOnly := core.Targ(func() {}).Name("flash-only")
	firmware := core.NewGroup("firmware", flashOnly)
	discover := core.Targ(func() {}).Name("discover")

	out := captureCompletionMulti(t, []any{firmware, discover},
		"app firmware flash-only discover ")
	// After chaining to discover, we should suggest roots again (both firmware and discover)
	if !strings.Contains(out, "firmware") || !strings.Contains(out, "discover") {
		t.Fatalf("expected root suggestions after chaining, got: %q", out)
	}
}

func TestCompletion_MultiRootUnknownRemaining(t *testing.T) {
	// Test multi-root mode where remaining args don't match any root
	// After firmware runs, "unknown" doesn't match any root so chain resolution stops
	// But suggestions still happen for current context (flash-only's parent has subcommands)
	flashOnly := core.Targ(func() {}).Name("flash-only")
	firmware := core.NewGroup("firmware", flashOnly)
	discover := core.Targ(func() {}).Name("discover")

	out := captureCompletionMulti(t, []any{firmware, discover},
		"app firmware flash-only unknown ")
	// The "unknown" remaining doesn't match any root, so followRemaining returns false
	// This means we should NOT suggest root commands (firmware, discover)
	// But we still get suggestions for the current subcommand context
	if strings.Contains(out, "firmware") || strings.Contains(out, "discover") {
		t.Fatalf("expected no root suggestions for unknown remaining, got: %q", out)
	}
	// Should still suggest caret (path reset) and flags
	if !strings.Contains(out, "^") {
		t.Fatalf("expected ^ suggestion, got: %q", out)
	}
}

func TestCompletion_MultipleRootsAtRootLevel(t *testing.T) {
	flashOnly := core.Targ(func() {}).Name("flash-only")
	firmware := core.NewGroup("firmware", flashOnly)
	discover := core.Targ(func() {}).Name("discover")

	out := captureCompletionMulti(t, []any{firmware, discover},
		"app ")
	if !strings.Contains(out, "firmware") || !strings.Contains(out, "discover") {
		t.Fatalf("expected root suggestions, got: %q", out)
	}
}

func TestCompletion_MultipleRootsWithPrefix(t *testing.T) {
	flashOnly := core.Targ(func() {}).Name("flash-only")
	firmware := core.NewGroup("firmware", flashOnly)
	discover := core.Targ(func() {}).Name("discover")

	out := captureCompletionMulti(t, []any{firmware, discover},
		"app f")
	if !strings.Contains(out, "firmware") {
		t.Fatalf("expected firmware suggestion, got: %q", out)
	}
}

func TestCompletion_PartialRootMatchSuggestsMatchingRoots(t *testing.T) {
	// "fir " (with trailing space) - doesn't match any root exactly but should suggest matching roots
	flashOnly := core.Targ(func() {}).Name("flash-only")
	firmware := core.NewGroup("firmware", flashOnly)
	discover := core.Targ(func() {}).Name("discover")

	out := captureCompletionMulti(t, []any{firmware, discover},
		"app fir ")
	if !strings.Contains(out, "firmware") {
		t.Fatalf("expected firmware suggestion for partial match, got: %q", out)
	}
}

// Tests for tokenization edge cases

func TestCompletion_QuotedArg(t *testing.T) {
	// Test that quoted arguments are handled properly
	target := core.Targ(func(_ EnumCmdArgs) {}).Name("enum-cmd")
	out := captureCompletion(t, target, `app --mode "de`)
	// Should suggest enum values since we're in a quoted string completing "de"
	if !strings.Contains(out, "dev") {
		t.Fatalf("expected dev suggestion with quoted arg, got: %q", out)
	}
}

func TestCompletion_SingleQuotedArg(t *testing.T) {
	// Test single quotes
	target := core.Targ(func(_ EnumCmdArgs) {}).Name("enum-cmd")

	out := captureCompletion(t, target, `app --mode 'de`)
	if !strings.Contains(out, "dev") {
		t.Fatalf("expected dev suggestion with single quoted arg, got: %q", out)
	}
}

func TestCompletion_SingleRootAtRoot(t *testing.T) {
	target := core.Targ(func(_ EnumCmdArgs) {}).Name("enum-cmd")

	out := captureCompletion(t, target, "app ")
	if !strings.Contains(out, "enum-cmd") {
		t.Fatalf("expected enum-cmd suggestion, got: %q", out)
	}
}

func TestCompletion_SingleRootWithRemaining(t *testing.T) {
	// Test single root mode with subcommand followed by extra remaining args
	// CompletionFirmwareRoot has FlashOnly subcommand; after that completes,
	// "extra" triggers followRemaining in single-root mode
	flashOnly := core.Targ(func() {}).Name("flash-only")
	firmware := core.NewGroup("firmware", flashOnly)

	out := captureCompletion(t, firmware, "app flash-only extra ")
	// In single root mode with remaining args, followRemaining sets currentNode back to root
	// and allows re-running. Should suggest flash-only (the subcommand) and flags
	if !strings.Contains(out, "flash-only") {
		t.Fatalf("expected subcommand suggestions after remaining args, got: %q", out)
	}
}

func TestCompletion_SuggestsEnumValuesAfterFlag(t *testing.T) {
	target := core.Targ(func(_ EnumCmdArgs) {}).Name("enum-cmd")

	out := captureCompletion(t, target, "app --mode ")
	if !strings.Contains(out, "dev") || !strings.Contains(out, "prod") {
		t.Fatalf("expected enum suggestions, got: %q", out)
	}
}

func TestCompletion_SuggestsEnumValuesAfterShortFlag(t *testing.T) {
	target := core.Targ(func(_ EnumCmdArgs) {}).Name("enum-cmd")

	out := captureCompletion(t, target, "app -m ")
	if !strings.Contains(out, "dev") || !strings.Contains(out, "prod") {
		t.Fatalf("expected enum suggestions for short flag, got: %q", out)
	}
}

func TestCompletion_SuggestsPositionalValues(t *testing.T) {
	// Simplified test - uses static enum instead of dynamic TagOptions
	target := core.Targ(func(_ PositionalCompletionCmdArgs) {}).Name("pos-cmd")

	out := captureCompletion(t, target, "app ")
	if !strings.Contains(out, "10") || !strings.Contains(out, "20") ||
		!strings.Contains(out, "30") {
		t.Fatalf("expected positional suggestions, got: %q", out)
	}
}

func TestCompletion_SuggestsRootsAfterCommand(t *testing.T) {
	flashOnly := core.Targ(func() {}).Name("flash-only")
	firmware := core.NewGroup("firmware", flashOnly)
	discover := core.Targ(func() {}).Name("discover")

	out := captureCompletionMulti(t, []any{firmware, discover},
		"app firmware flash-only d")
	if !strings.Contains(out, "discover") {
		t.Fatalf("expected discover suggestion, got: %q", out)
	}
}

func TestCompletion_TagOptionsOverride(t *testing.T) {
	// Changed to use static enum (alpha|beta) since dynamic TagOptions is struct-only
	target := core.Targ(func(_ EnumOverrideCmdArgs) {}).Name("enum-override-cmd")
	out := captureCompletion(t, target, "app --mode ")
	// Static enum is alpha|beta
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("expected static enum values, got: %q", out)
	}
}

func TestCompletion_UnknownRootPrefix(t *testing.T) {
	flashOnly := core.Targ(func() {}).Name("flash-only")
	firmware := core.NewGroup("firmware", flashOnly)
	discover := core.Targ(func() {}).Name("discover")

	out := captureCompletionMulti(t, []any{firmware, discover},
		"app xyz")
	// Should not suggest anything since no match
	if out != "" {
		t.Fatalf("expected no suggestions for unknown prefix, got: %q", out)
	}
}

func TestCompletion_VariadicFlagSkipsMultipleValues(t *testing.T) {
	target := core.Targ(func(_ VariadicFlagCmdArgs) {}).Name("variadic-cmd")
	out := captureCompletion(t, target, "app --files a.txt b.txt ")
	// Should suggest positional enum values after skipping variadic flag values
	if !strings.Contains(out, "build") || !strings.Contains(out, "test") {
		t.Fatalf("expected positional enum suggestions after variadic flag values, got: %q", out)
	}
}

func TestPrintCompletionScriptPlaceholders(t *testing.T) {
	cases := []string{"bash", "zsh", "fish"}
	for _, shell := range cases {
		out := captureStdout(t, func() {
			err := core.PrintCompletionScript(shell, "demo")
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

// captureCompletion runs __complete with a single target and returns stdout.
func captureCompletion(t *testing.T, target any, input string) string {
	t.Helper()

	return captureCompletionMulti(t, []any{target}, input)
}

// captureCompletionMulti runs __complete with multiple targets and returns stdout.
func captureCompletionMulti(t *testing.T, targets []any, input string) string {
	t.Helper()

	return captureStdout(t, func() {
		args := []string{"app", "__complete", input}

		_, err := core.Execute(args, targets...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
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
