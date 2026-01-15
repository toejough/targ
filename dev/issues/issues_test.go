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

// Tests for issue file parsing (moved from internal/issuefile).

func TestParseAndUpdateStatus(t *testing.T) {
	content := `# Issue Tracker

## Backlog

### 1. Test Issue

#### Universal

**Description**
Something
`
	file, err := parseIssueFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	iss, _ := file.find(1)
	if iss == nil {
		t.Fatal("expected issue")
	}
	block := issueBlockLines(file.lines, *iss)
	block = updateIssueStatus(block, "backlog")
	if parseStatus(block) != "backlog" {
		t.Fatalf("expected status to be added, got %q", parseStatus(block))
	}
}

func TestParse_StopsAtSectionHeader(t *testing.T) {
	content := strings.Join([]string{
		"## Backlog",
		"",
		"### 1. First",
		"",
		"**Status**",
		"backlog",
		"",
		"## Done",
		"",
		"Completed issues.",
		"",
		"### 2. Done",
		"",
		"**Status**",
		"done",
	}, "\n")
	file, err := parseIssueFile(content)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(file.issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(file.issues))
	}
	if file.issues[0].Number != 1 {
		t.Fatalf("expected first issue number 1, got %d", file.issues[0].Number)
	}
	if file.issues[1].Number != 2 {
		t.Fatalf("expected second issue number 2, got %d", file.issues[1].Number)
	}
}

func TestUpdateSectionField(t *testing.T) {
	lines := []string{
		"### 1. Test",
		"",
		"**Description**",
		"Old",
		"",
		"**Priority**",
		"Low",
	}
	lines = updateSectionField(lines, "Description", "New")
	if got := testSectionValue(lines, "Description"); got != "New" {
		t.Fatalf("expected updated description, got %q", got)
	}
	lines = updateSectionField(lines, "Acceptance", "OK")
	if got := testSectionValue(lines, "Acceptance"); got != "OK" {
		t.Fatalf("expected inserted acceptance, got %q", got)
	}
	lines = updateSectionField(lines, "Details", "Steps")
	if got := testSectionValue(lines, "Details"); got != "Steps" {
		t.Fatalf("expected inserted details, got %q", got)
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

func testSectionValue(lines []string, field string) string {
	header := "**" + field + "**"
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == header {
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				return strings.TrimSpace(lines[j])
			}
		}
	}
	return ""
}
