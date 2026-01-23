//go:build targ

package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Issue struct {
	Number  int
	Title   string
	Section string
	Status  string
	Start   int
	End     int
}

type IssueFile struct {
	lines  []string
	issues []Issue
}

func (f *IssueFile) Find(number int) (*Issue, int) {
	for i := range f.issues {
		if f.issues[i].Number == number {
			return &f.issues[i], i
		}
	}
	return nil, -1
}

func (f *IssueFile) Insert(section string, issueLines []string) error {
	idx := insertIndex(f.lines, section)
	if idx < 0 {
		return fmt.Errorf("section %s not found", section)
	}
	lines := make([]string, 0, len(f.lines)+len(issueLines)+1)
	lines = append(lines, f.lines[:idx]...)
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, issueLines...)
	if len(issueLines) > 0 && strings.TrimSpace(issueLines[len(issueLines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, f.lines[idx:]...)
	f.lines = lines
	return nil
}

func (f *IssueFile) Issues() []Issue {
	return f.issues
}

func (f *IssueFile) Lines() []string {
	return f.lines
}

func (f *IssueFile) Remove(iss Issue) {
	f.lines = append(f.lines[:iss.Start], f.lines[iss.End:]...)
}

func (f *IssueFile) SetLines(lines []string) {
	f.lines = lines
}

func (f *IssueFile) UpdateIssue(number int, updates IssueUpdates) (Issue, error) {
	iss, _ := f.Find(number)
	if iss == nil {
		return Issue{}, fmt.Errorf("issue %d not found", number)
	}
	block := IssueBlockLines(f.lines, *iss)
	if updates.Status != nil {
		block = UpdateIssueStatus(block, *updates.Status)
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
	f.Remove(*iss)
	section := iss.Section
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
	return *iss, nil
}

type IssueUpdates struct {
	Status      *string
	Description *string
	Priority    *string
	Acceptance  *string
	Details     *string
}

// FindIssueFile searches downward from the current directory for a file named
// issues.md. Returns the path if found, or the original path if not found.
func FindIssueFile(path string) string {
	// If the file exists at the given path, use it
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Search downward for issues.md
	target := filepath.Base(path)
	var found string

	_ = filepath.WalkDir(".", func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if d.IsDir() {
			// Skip hidden directories and common non-source directories
			name := d.Name()
			if name != "." && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == target {
			found = p
			return filepath.SkipAll
		}
		return nil
	})

	if found != "" {
		return found
	}
	return path
}

func InsertAfter(lines []string, idx int, insert []string) []string {
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

func IssueBlockLines(lines []string, iss Issue) []string {
	block := make([]string, iss.End-iss.Start)
	copy(block, lines[iss.Start:iss.End])
	return block
}

func LoadIssues(path string) (*IssueFile, []Issue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	file, err := ParseIssueFile(string(data))
	if err != nil {
		return nil, nil, err
	}
	return file, file.issues, nil
}

func NormalizePriority(priority string) string {
	normalized := strings.ToLower(strings.TrimSpace(priority))
	switch normalized {
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	default:
		return priority
	}
}

func NormalizeStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "in-progress", "in_progress", "inprogress":
		return "in progress"
	default:
		return normalized
	}
}

func ParseHeader(line string) (int, string) {
	header := strings.TrimSpace(strings.TrimPrefix(line, "### "))
	parts := strings.SplitN(header, ".", 2)
	if len(parts) < 2 {
		return 0, header
	}
	number, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	title := strings.TrimSpace(parts[1])
	return number, title
}

func ParseIssueFile(content string) (*IssueFile, error) {
	lines := strings.Split(content, "\n")
	file := &IssueFile{lines: lines}

	section := ""
	var current *Issue
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current != nil {
				current.End = i
				current.Status = ParseStatus(lines[current.Start:current.End])
				file.issues = append(file.issues, *current)
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
				current.Status = ParseStatus(lines[current.Start:current.End])
				file.issues = append(file.issues, *current)
			}
			number, title := ParseHeader(line)
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
		current.Status = ParseStatus(lines[current.Start:current.End])
		file.issues = append(file.issues, *current)
	}
	return file, nil
}

func ParseStatus(lines []string) string {
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

func UpdateIssueStatus(lines []string, status string) []string {
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
			return InsertAfter(lines, i-1, insert)
		}
	}
	return append(lines, "", header, value)
}

func WriteIssues(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
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
