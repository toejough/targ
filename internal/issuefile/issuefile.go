package issuefile

import (
	"fmt"
	"strconv"
	"strings"
)

type File struct {
	Lines  []string
	Issues []Issue
}

type Issue struct {
	Number  int
	Title   string
	Section string
	Status  string
	Start   int
	End     int
}

func Parse(content string) (*File, error) {
	lines := strings.Split(content, "\n")
	file := &File{Lines: lines}

	section := ""
	var current *Issue
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current != nil {
				current.End = i
				current.Status = parseStatus(lines[current.Start:current.End])
				file.Issues = append(file.Issues, *current)
				current = nil
			}
			header := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			switch strings.ToLower(header) {
			case "backlog":
				section = "backlog"
			case "done":
				section = "done"
			default:
				section = ""
			}
		}

		if strings.HasPrefix(line, "### ") {
			if current != nil {
				current.End = i
				current.Status = parseStatus(lines[current.Start:current.End])
				file.Issues = append(file.Issues, *current)
			}
			number, title := parseHeader(line)
			current = &Issue{
				Number:  number,
				Title:   title,
				Section: section,
				Start:   i,
			}
		}
	}
	if current != nil {
		current.End = len(lines)
		current.Status = parseStatus(lines[current.Start:current.End])
		file.Issues = append(file.Issues, *current)
	}
	return file, nil
}

func parseHeader(line string) (int, string) {
	header := strings.TrimSpace(strings.TrimPrefix(line, "### "))
	parts := strings.SplitN(header, ".", 2)
	if len(parts) < 2 {
		return 0, header
	}
	number, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	title := strings.TrimSpace(parts[1])
	return number, title
}

func parseStatus(lines []string) string {
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "**Status**" {
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

func (f *File) Find(number int) (*Issue, int) {
	for i := range f.Issues {
		if f.Issues[i].Number == number {
			return &f.Issues[i], i
		}
	}
	return nil, -1
}

func (f *File) Remove(issue Issue) {
	f.Lines = append(f.Lines[:issue.Start], f.Lines[issue.End:]...)
}

func (f *File) Insert(section string, issueLines []string) error {
	idx := insertIndex(f.Lines, section)
	if idx < 0 {
		return fmt.Errorf("section %s not found", section)
	}
	lines := make([]string, 0, len(f.Lines)+len(issueLines)+1)
	lines = append(lines, f.Lines[:idx]...)
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, issueLines...)
	if len(issueLines) > 0 && strings.TrimSpace(issueLines[len(issueLines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, f.Lines[idx:]...)
	f.Lines = lines
	return nil
}

func (f *File) UpdateIssue(number int, updates IssueUpdates) (Issue, error) {
	issue, _ := f.Find(number)
	if issue == nil {
		return Issue{}, fmt.Errorf("issue %d not found", number)
	}
	block := IssueBlockLines(f.Lines, *issue)
	if updates.Status != nil {
		block = UpdateStatus(block, *updates.Status)
	}
	if updates.Description != nil {
		block = UpdateSectionField(block, "Description", *updates.Description)
	}
	if updates.Priority != nil {
		block = UpdateSectionField(block, "Priority", *updates.Priority)
	}
	if updates.Acceptance != nil {
		block = UpdateSectionField(block, "Acceptance", *updates.Acceptance)
	}
	if updates.Details != nil {
		block = UpdateSectionField(block, "Details", *updates.Details)
	}
	f.Remove(*issue)
	section := issue.Section
	if updates.Status != nil {
		if strings.EqualFold(*updates.Status, "done") {
			section = "done"
		} else {
			section = "backlog"
		}
	}
	if err := f.Insert(section, block); err != nil {
		return Issue{}, err
	}
	return *issue, nil
}

type IssueUpdates struct {
	Status      *string
	Description *string
	Priority    *string
	Acceptance  *string
	Details     *string
}

func UpdateSectionField(lines []string, field string, value string) []string {
	header := fmt.Sprintf("**%s**", field)
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == header {
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				lines[j] = value
				return lines
			}
			insert := []string{header, value, ""}
			return insertAfter(lines, i-1, insert)
		}
	}
	return append(lines, "", header, value)
}

func insertAfter(lines []string, idx int, insert []string) []string {
	if idx < 0 {
		return append(insert, lines...)
	}
	if idx >= len(lines)-1 {
		return append(lines, insert...)
	}
	out := make([]string, 0, len(lines)+len(insert))
	out = append(out, lines[:idx+1]...)
	out = append(out, insert...)
	out = append(out, lines[idx+1:]...)
	return out
}

func insertIndex(lines []string, section string) int {
	switch section {
	case "backlog":
		for i, line := range lines {
			if strings.HasPrefix(line, "## Done") {
				return i
			}
		}
		return len(lines)
	case "done":
		return len(lines)
	default:
		return -1
	}
}

func UpdateStatus(lines []string, status string) []string {
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "**Status**" {
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				lines[j] = status
				return lines
			}
			lines = append(lines[:i+1], append([]string{status}, lines[i+1:]...)...)
			return lines
		}
	}
	insertAt := 1
	for i, line := range lines {
		if strings.HasPrefix(line, "#### Universal") {
			insertAt = i + 1
			break
		}
	}
	block := []string{"", "**Status**", status}
	lines = append(lines[:insertAt], append(block, lines[insertAt:]...)...)
	return lines
}

func IssueBlockLines(lines []string, issue Issue) []string {
	block := make([]string, issue.End-issue.Start)
	copy(block, lines[issue.Start:issue.End])
	return block
}
