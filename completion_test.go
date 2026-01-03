package commander

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

type EnumCmd struct {
	Mode string `commander:"flag,enum=dev|prod,short=m"`
	Kind string `commander:"flag,enum=fast|slow"`
}

func (c *EnumCmd) Run() {}

func TestEnumValuesForArg_LongFlag(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	values, ok := enumValuesForArg(cmd, []string{"--mode"}, "", true)
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

	values, ok := enumValuesForArg(cmd, []string{"-m"}, "", true)
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

	if values, ok := enumValuesForArg(cmd, []string{"--unknown"}, "", true); ok {
		t.Fatalf("expected no enum values, got %v", values)
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
		doCompletion([]*CommandNode{cmd}, "app --mode ")
	})
	if !strings.Contains(out, "dev") || !strings.Contains(out, "prod") {
		t.Fatalf("expected enum suggestions, got: %q", out)
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
