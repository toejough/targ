package issuefile

import "testing"

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
