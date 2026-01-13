//go:build targ

package issues

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListOutputsHeaderAndColumns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "issues.md")
	content := strings.Join([]string{
		"# Issue Tracker",
		"",
		"## Backlog",
		"",
		"### 1. First",
		"",
		"**Status**",
		"backlog",
		"",
		"**Description**",
		"One",
		"",
		"### 2. Second",
		"",
		"**Status**",
		"done",
		"",
		"**Description**",
		"Two",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	cmd := &List{File: path}
	output := captureStdout(t, func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("unexpected run error: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected header + rows, got: %q", output)
	}
	if lines[0] != "ID\tStatus\tTitle" {
		t.Fatalf("unexpected header: %q", lines[0])
	}
	if !strings.Contains(lines[1], "1\tbacklog\tFirst") {
		t.Fatalf("unexpected row: %q", lines[1])
	}
	if !strings.Contains(lines[2], "2\tdone\tSecond") {
		t.Fatalf("unexpected row: %q", lines[2])
	}
}

func TestNormalizePriority(t *testing.T) {
	cases := map[string]string{
		"low":    "Low",
		"LOW":    "Low",
		"Medium": "Medium",
		"high":   "High",
		"":       "",
		"custom": "custom",
	}
	for input, want := range cases {
		if got := normalizePriority(input); got != want {
			t.Fatalf("normalizePriority(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]string{
		"backlog":     "backlog",
		"Done":        "done",
		"in-progress": "in progress",
		"in_progress": "in progress",
		"inprogress":  "in progress",
		" review ":    "review",
	}
	for input, want := range cases {
		if got := normalizeStatus(input); got != want {
			t.Fatalf("normalizeStatus(%q) = %q, want %q", input, got, want)
		}
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
