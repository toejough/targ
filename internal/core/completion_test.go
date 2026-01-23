package core_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/toejough/targ/internal/core"
)

// Test command types for completion testing.

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

func (c *EnumOverrideCmd) TagOptions(field string, opts core.TagOptions) (core.TagOptions, error) {
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

func (c *PositionalCompletionCmd) TagOptions(
	field string,
	opts core.TagOptions,
) (core.TagOptions, error) {
	if field != "ID" {
		return opts, nil
	}

	if c.Status == "cancelled" {
		opts.Enum = "40|41"
	}

	return opts, nil
}

type VariadicFlagCmd struct {
	Files  []string `targ:"flag"`
	Target string   `targ:"positional,enum=build|test"`
}

func (c *VariadicFlagCmd) Run() {}

func TestCompletion_SuggestsEnumValuesAfterFlag(t *testing.T) {

	out := captureCompletion(t, &EnumCmd{}, "app --mode ")
	if !strings.Contains(out, "dev") || !strings.Contains(out, "prod") {
		t.Fatalf("expected enum suggestions, got: %q", out)
	}
}

func TestCompletion_SuggestsEnumValuesAfterShortFlag(t *testing.T) {

	out := captureCompletion(t, &EnumCmd{}, "app -m ")
	if !strings.Contains(out, "dev") || !strings.Contains(out, "prod") {
		t.Fatalf("expected enum suggestions for short flag, got: %q", out)
	}
}

func TestCompletion_SuggestsPositionalValues(t *testing.T) {

	out := captureCompletion(t, &PositionalCompletionCmd{}, "app --status cancelled ")
	if !strings.Contains(out, "40") || !strings.Contains(out, "41") {
		t.Fatalf("expected positional suggestions, got: %q", out)
	}
}

func TestCompletion_SuggestsRootsAfterCommand(t *testing.T) {

	out := captureCompletionMulti(t, []any{&CompletionFirmwareRoot{}, &CompletionDiscoverRoot{}},
		"app firmware flash-only d")
	if !strings.Contains(out, "discover") {
		t.Fatalf("expected discover suggestion, got: %q", out)
	}
}

func TestCompletion_CaretSuggestion(t *testing.T) {

	out := captureCompletionMulti(t, []any{&CompletionFirmwareRoot{}, &CompletionDiscoverRoot{}},
		"app firmware flash-only ")
	if !strings.Contains(out, "^") {
		t.Fatalf("expected ^ suggestion, got: %q", out)
	}
}

func TestCompletion_ChainedRootCommands(t *testing.T) {

	out := captureCompletionMulti(t, []any{&CompletionFirmwareRoot{}, &CompletionDiscoverRoot{}},
		"app firmware discover ")
	// After chaining through both commands, should suggest roots again
	if !strings.Contains(out, "firmware") || !strings.Contains(out, "discover") {
		t.Fatalf("expected root suggestions after chained commands, got: %q", out)
	}
}

func TestCompletion_FlagSuggestion(t *testing.T) {

	out := captureCompletion(t, &EnumCmd{}, "app --")
	if !strings.Contains(out, "--mode") {
		t.Fatalf("expected --mode flag suggestion, got: %q", out)
	}
}

func TestCompletion_MultipleRootsAtRootLevel(t *testing.T) {

	out := captureCompletionMulti(t, []any{&CompletionFirmwareRoot{}, &CompletionDiscoverRoot{}},
		"app ")
	if !strings.Contains(out, "firmware") || !strings.Contains(out, "discover") {
		t.Fatalf("expected root suggestions, got: %q", out)
	}
}

func TestCompletion_MultipleRootsWithPrefix(t *testing.T) {

	out := captureCompletionMulti(t, []any{&CompletionFirmwareRoot{}, &CompletionDiscoverRoot{}},
		"app f")
	if !strings.Contains(out, "firmware") {
		t.Fatalf("expected firmware suggestion, got: %q", out)
	}
}

func TestCompletion_SingleRootAtRoot(t *testing.T) {

	out := captureCompletion(t, &EnumCmd{}, "app ")
	if !strings.Contains(out, "enum-cmd") {
		t.Fatalf("expected enum-cmd suggestion, got: %q", out)
	}
}

func TestCompletion_UnknownRootPrefix(t *testing.T) {

	out := captureCompletionMulti(t, []any{&CompletionFirmwareRoot{}, &CompletionDiscoverRoot{}},
		"app xyz")
	// Should not suggest anything since no match
	if out != "" {
		t.Fatalf("expected no suggestions for unknown prefix, got: %q", out)
	}
}

func TestCompletion_VariadicFlagSkipsMultipleValues(t *testing.T) {

	out := captureCompletion(t, &VariadicFlagCmd{}, "app --files a.txt b.txt ")
	// Should suggest positional enum values after skipping variadic flag values
	if !strings.Contains(out, "build") || !strings.Contains(out, "test") {
		t.Fatalf("expected positional enum suggestions after variadic flag values, got: %q", out)
	}
}

func TestCompletion_TagOptionsOverride(t *testing.T) {

	out := captureCompletion(t, &EnumOverrideCmd{}, "app --mode ")
	// TagOptions overrides enum to alpha|beta
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("expected overridden enum values, got: %q", out)
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
