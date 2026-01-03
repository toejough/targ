package issuefile

import (
	"strings"
	"testing"
)

func TestParseAndUpdateStatus(t *testing.T) {
	content := `# Issue Tracker

## Backlog

### 1. Test Issue

#### Universal

**Description**
Something
`
	file, err := Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	issue, _ := file.Find(1)
	if issue == nil {
		t.Fatal("expected issue")
	}
	block := IssueBlockLines(file.Lines, *issue)
	block = UpdateStatus(block, "backlog")
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
	file, err := Parse(content)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(file.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(file.Issues))
	}
	if file.Issues[0].Number != 1 {
		t.Fatalf("expected first issue number 1, got %d", file.Issues[0].Number)
	}
	if file.Issues[1].Number != 2 {
		t.Fatalf("expected second issue number 2, got %d", file.Issues[1].Number)
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
	lines = UpdateSectionField(lines, "Description", "New")
	if got := sectionValue(lines, "Description"); got != "New" {
		t.Fatalf("expected updated description, got %q", got)
	}
	lines = UpdateSectionField(lines, "Acceptance", "OK")
	if got := sectionValue(lines, "Acceptance"); got != "OK" {
		t.Fatalf("expected inserted acceptance, got %q", got)
	}
	lines = UpdateSectionField(lines, "Details", "Steps")
	if got := sectionValue(lines, "Details"); got != "Steps" {
		t.Fatalf("expected inserted details, got %q", got)
	}
}

func sectionValue(lines []string, field string) string {
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
