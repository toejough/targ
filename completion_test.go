package targ

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

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

func TestPrintCompletionScriptPlaceholders(t *testing.T) {
	cases := []string{"bash", "zsh", "fish"}
	for _, shell := range cases {
		out := captureStdout(t, func() {
			if err := PrintCompletionScript(shell, "demo"); err != nil {
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

func TestCompletionSuggestsEnumValuesAfterFlag(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := captureStdout(t, func() {
		if err := doCompletion([]*commandNode{cmd}, "app --mode "); err != nil {
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
		if err := doCompletion([]*commandNode{cmd}, "app --status cancelled "); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "40") || !strings.Contains(out, "41") {
		t.Fatalf("expected positional suggestions, got: %q", out)
	}
}

type CompletionFirmwareRoot struct {
	FlashOnly *CompletionFlashOnly `targ:"subcommand=flash-only"`
}

func (c *CompletionFirmwareRoot) Name() string { return "firmware" }

type CompletionFlashOnly struct{}

func (c *CompletionFlashOnly) Run() {}

type CompletionDiscoverRoot struct{}

func (c *CompletionDiscoverRoot) Name() string { return "discover" }

func (c *CompletionDiscoverRoot) Run() {}

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
		if err := doCompletion([]*commandNode{firmware, discover}, "app firmware flash-only d"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "discover") {
		t.Fatalf("expected discover suggestion, got: %q", out)
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
