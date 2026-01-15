//go:build targ

// Package issues provides issue list tooling for targ.
package issues

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Create struct {
	File       string `targ:"flag,default=issues.md,desc=Issue file to update"`
	GitHub     bool   `targ:"flag,desc=Create issue on GitHub instead of locally"`
	Title      string `targ:"flag,required,desc=Issue title"`
	Status     string `targ:"flag,default=backlog,desc=Initial status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
	Desc       string `targ:"flag,name=description,default=TBD,desc=Issue description"`
	Priority   string `targ:"flag,default=Low,desc=Priority,enum=low|medium|high"`
	Acceptance string `targ:"flag,default=TBD,desc=Acceptance criteria"`
}

func (c *Create) Description() string {
	return "Create a new issue locally or on GitHub"
}

func (c *Create) Run() error {
	if c.GitHub {
		num, err := createGitHubIssue(c.Title, c.Desc)
		if err != nil {
			return err
		}
		fmt.Printf("Created GitHub issue: %s\n", formatIssueID("github", num))
		return nil
	}

	content, issues, err := loadIssues(c.File)
	if err != nil {
		return err
	}
	maxID := 0
	for _, issue := range issues {
		if issue.Number > maxID {
			maxID = issue.Number
		}
	}
	newID := maxID + 1

	status := normalizeStatus(c.Status)
	priority := normalizePriority(c.Priority)
	block := []string{
		fmt.Sprintf("### %d. %s", newID, c.Title),
		"",
		"#### Universal",
		"",
		"**Status**",
		status,
		"",
		"**Description**",
		c.Desc,
		"",
		"#### Planning",
		"",
		"**Priority**",
		priority,
		"",
		"**Acceptance**",
		c.Acceptance,
	}

	section := "backlog"
	if strings.EqualFold(status, "done") {
		section = "done"
	}
	if err := content.insert(section, block); err != nil {
		return err
	}
	fmt.Printf("Created local issue: %s\n", formatIssueID("local", newID))
	return writeIssues(c.File, content.lines)
}

type Dedupe struct {
	File string `targ:"flag,default=issues.md,desc=Issue file to update"`
}

func (c *Dedupe) Description() string {
	return "Remove duplicate done issues from backlog"
}

func (c *Dedupe) Run() error {
	content, _, err := loadIssues(c.File)
	if err != nil {
		return err
	}
	file := content
	issues := file.issues
	done := map[int]bool{}
	for _, iss := range issues {
		if iss.Section == "done" {
			done[iss.Number] = true
		}
	}
	for i := len(issues) - 1; i >= 0; i-- {
		iss := issues[i]
		if iss.Section == "backlog" && done[iss.Number] {
			file.remove(iss)
		}
	}
	return writeIssues(c.File, file.lines)
}

type List struct {
	File   string `targ:"flag,default=issues.md,desc=Issue file to read"`
	Status string `targ:"flag,desc=Filter by status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
	Query  string `targ:"flag,desc=Case-insensitive title filter"`
	Source string `targ:"flag,default=all,desc=Issue source,enum=all|local|github"`
}

func (c *List) Description() string {
	return "List issues from local file and/or GitHub"
}

func (c *List) Run() error {
	type displayIssue struct {
		ID     string
		Status string
		Title  string
	}
	var all []displayIssue

	// Load local issues
	if c.Source == "all" || c.Source == "local" {
		file, issues, err := loadIssues(c.File)
		if err != nil && c.Source == "local" {
			return err
		}
		_ = file
		for _, issue := range issues {
			all = append(all, displayIssue{
				ID:     formatIssueID("local", issue.Number),
				Status: normalizeStatus(issue.Status),
				Title:  issue.Title,
			})
		}
	}

	// Load GitHub issues
	if c.Source == "all" || c.Source == "github" {
		ghIssues, err := listGitHubIssues("")
		if err != nil && c.Source == "github" {
			return err
		}
		if err == nil {
			for _, issue := range ghIssues {
				all = append(all, displayIssue{
					ID:     formatIssueID("github", issue.Number),
					Status: ghStateToStatus(issue.State),
					Title:  issue.Title,
				})
			}
		}
	}

	// Filter by status
	wantStatus := normalizeStatus(c.Status)
	var filtered []displayIssue
	for _, issue := range all {
		if wantStatus != "" && !strings.EqualFold(issue.Status, wantStatus) {
			continue
		}
		if c.Query != "" && !strings.Contains(strings.ToLower(issue.Title), strings.ToLower(c.Query)) {
			continue
		}
		filtered = append(filtered, issue)
	}

	fmt.Println("ID\tStatus\tTitle")
	for _, issue := range filtered {
		fmt.Printf("%s\t%s\t%s\n", issue.ID, issue.Status, issue.Title)
	}
	return nil
}

type Move struct {
	File   string `targ:"flag,default=issues.md,desc=Issue file to update"`
	ID     string `targ:"positional,required,desc=Issue ID (e.g. 5 for local or gh#5 for GitHub)"`
	Status string `targ:"flag,required,desc=New status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
}

func (c *Move) Description() string {
	return "Move a local or GitHub issue to a new status"
}

func (c *Move) Run() error {
	source, number, err := parseIssueID(c.ID)
	if err != nil {
		return err
	}

	status := normalizeStatus(c.Status)

	if source == "github" {
		// For GitHub, moving to done/cancelled means closing, otherwise reopening
		if status == "done" || status == "cancelled" {
			if err := closeGitHubIssue(number); err != nil {
				return err
			}
		} else {
			if err := reopenGitHubIssue(number); err != nil {
				return err
			}
		}
		fmt.Printf("Moved GitHub issue %s to %s\n", formatIssueID("github", number), status)
		return nil
	}

	// Handle local issue
	content, _, err := loadIssues(c.File)
	if err != nil {
		return err
	}
	file := content
	iss, _ := file.find(number)
	if iss == nil {
		return fmt.Errorf("issue %d not found", number)
	}
	block := issueBlockLines(file.lines, *iss)
	block = updateIssueStatus(block, status)
	file.remove(*iss)

	section := "backlog"
	if strings.EqualFold(status, "done") {
		section = "done"
	}
	if err := file.insert(section, block); err != nil {
		return err
	}

	fmt.Printf("Moved local issue %s to %s\n", formatIssueID("local", number), status)
	return writeIssues(c.File, file.lines)
}

type Update struct {
	File       string `targ:"flag,default=issues.md,desc=Issue file to update"`
	ID         string `targ:"positional,required,desc=Issue ID (e.g. 5 for local or gh#5 for GitHub)"`
	Status     string `targ:"flag,desc=New status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
	Desc       string `targ:"flag,name=description,desc=Description text"`
	Priority   string `targ:"flag,desc=Priority,enum=low|medium|high"`
	Acceptance string `targ:"flag,desc=Acceptance criteria"`
	Details    string `targ:"flag,desc=Implementation details"`
}

func (c *Update) Description() string {
	return "Update a local or GitHub issue"
}

func (c *Update) Run() error {
	source, number, err := parseIssueID(c.ID)
	if err != nil {
		return err
	}

	if source == "github" {
		// Handle GitHub issue update
		ghUpdates := gitHubUpdates{}
		if c.Desc != "" {
			ghUpdates.Body = &c.Desc
		}
		// GitHub doesn't have priority/acceptance/details fields directly
		// For status changes, we close/reopen
		if c.Status != "" {
			status := normalizeStatus(c.Status)
			if status == "done" || status == "cancelled" {
				if err := closeGitHubIssue(number); err != nil {
					return err
				}
			} else {
				if err := reopenGitHubIssue(number); err != nil {
					return err
				}
			}
		}
		if ghUpdates.Body != nil {
			if err := updateGitHubIssue(number, ghUpdates); err != nil {
				return err
			}
		}
		fmt.Printf("Updated GitHub issue: %s\n", formatIssueID("github", number))
		return nil
	}

	// Handle local issue update
	file, _, err := loadIssues(c.File)
	if err != nil {
		return err
	}

	updates := issueUpdates{}
	if c.Status != "" {
		status := normalizeStatus(c.Status)
		updates.Status = &status
	}
	if c.Desc != "" {
		updates.Description = &c.Desc
	}
	if c.Priority != "" {
		priority := normalizePriority(c.Priority)
		updates.Priority = &priority
	}
	if c.Acceptance != "" {
		updates.Acceptance = &c.Acceptance
	}
	if c.Details != "" {
		updates.Details = &c.Details
	}

	if updates == (issueUpdates{}) {
		return fmt.Errorf("no updates provided")
	}

	if _, err := file.updateIssue(number, updates); err != nil {
		return err
	}
	fmt.Printf("Updated local issue: %s\n", formatIssueID("local", number))
	return writeIssues(c.File, file.lines)
}

type Validate struct {
	File string `targ:"flag,default=issues.md,desc=Issue file to validate"`
}

func (c *Validate) Description() string {
	return "Validate issue formatting and structure"
}

func (c *Validate) Run() error {
	_, issues, err := loadIssues(c.File)
	if err != nil {
		return err
	}
	seen := map[int]string{}
	var errs []string
	for _, issue := range issues {
		if issue.Number == 0 {
			continue
		}
		if other, ok := seen[issue.Number]; ok {
			errs = append(errs, fmt.Sprintf("duplicate issue %d (%s vs %s)", issue.Number, other, issue.Title))
			continue
		}
		seen[issue.Number] = issue.Title
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

func loadIssues(path string) (*issueFile, []issue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	file, err := parseIssueFile(string(data))
	if err != nil {
		return nil, nil, err
	}
	return file, file.issues, nil
}

func normalizePriority(priority string) string {
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

func normalizeStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "in-progress", "in_progress", "inprogress":
		return "in progress"
	default:
		return normalized
	}
}

func writeIssues(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// Issue file parsing and manipulation types/functions.

type issueFile struct {
	lines  []string
	issues []issue
}

func (f *issueFile) find(number int) (*issue, int) {
	for i := range f.issues {
		if f.issues[i].Number == number {
			return &f.issues[i], i
		}
	}
	return nil, -1
}

func (f *issueFile) insert(section string, issueLines []string) error {
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

func (f *issueFile) remove(iss issue) {
	f.lines = append(f.lines[:iss.Start], f.lines[iss.End:]...)
}

func (f *issueFile) updateIssue(number int, updates issueUpdates) (issue, error) {
	iss, _ := f.find(number)
	if iss == nil {
		return issue{}, fmt.Errorf("issue %d not found", number)
	}
	block := issueBlockLines(f.lines, *iss)
	if updates.Status != nil {
		block = updateIssueStatus(block, *updates.Status)
	}
	if updates.Description != nil {
		block = updateSectionField(block, "Description", *updates.Description)
	}
	if updates.Priority != nil {
		block = updateSectionField(block, "Priority", *updates.Priority)
	}
	if updates.Acceptance != nil {
		block = updateSectionField(block, "Acceptance", *updates.Acceptance)
	}
	if updates.Details != nil {
		block = updateSectionField(block, "Details", *updates.Details)
	}
	f.remove(*iss)
	section := iss.Section
	if updates.Status != nil {
		if strings.EqualFold(*updates.Status, "done") {
			section = "done"
		} else {
			section = "backlog"
		}
	}
	if err := f.insert(section, block); err != nil {
		return issue{}, err
	}
	return *iss, nil
}

type issue struct {
	Number  int
	Title   string
	Section string
	Status  string
	Start   int
	End     int
}

type issueUpdates struct {
	Status      *string
	Description *string
	Priority    *string
	Acceptance  *string
	Details     *string
}

func issueBlockLines(lines []string, iss issue) []string {
	block := make([]string, iss.End-iss.Start)
	copy(block, lines[iss.Start:iss.End])
	return block
}

func parseIssueFile(content string) (*issueFile, error) {
	lines := strings.Split(content, "\n")
	file := &issueFile{lines: lines}

	section := ""
	var current *issue
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current != nil {
				current.End = i
				current.Status = parseStatus(lines[current.Start:current.End])
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
				current.Status = parseStatus(lines[current.Start:current.End])
				file.issues = append(file.issues, *current)
			}
			number, title := parseHeader(line)
			current = &issue{
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
		file.issues = append(file.issues, *current)
	}
	return file, nil
}

func updateSectionField(lines []string, field string, value string) []string {
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

func updateIssueStatus(lines []string, status string) []string {
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
