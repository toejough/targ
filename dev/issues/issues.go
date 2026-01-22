//go:build targ

// Package issues provides issue list tooling for targ.
package issues

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/toejough/targ"
)

func init() {
	targ.Register(
		Create,
		Dedupe,
		List,
		Move,
		Update,
		Validate,
	)
}

// Exported variables.
var (
	Create   = targ.Targ(create).Description("Create a new issue locally or on GitHub")
	Dedupe   = targ.Targ(dedupe).Description("Remove duplicate done issues from backlog")
	List     = targ.Targ(list).Description("List issues from local file and/or GitHub")
	Move     = targ.Targ(move).Description("Move a local or GitHub issue to a new status")
	Update   = targ.Targ(update).Description("Update a local or GitHub issue")
	Validate = targ.Targ(validate).Description("Validate issue formatting and structure")
)

// CreateArgs are arguments for the create command.
type CreateArgs struct {
	File       string `targ:"flag,default=issues.md,desc=Issue file to update"`
	GitHub     bool   `targ:"flag,desc=Create issue on GitHub instead of locally"`
	Title      string `targ:"flag,required,desc=Issue title"`
	Status     string `targ:"flag,default=backlog,desc=Initial status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
	Desc       string `targ:"flag,name=description,default=TBD,desc=Issue description"`
	Priority   string `targ:"flag,default=Low,desc=Priority,enum=low|medium|high"`
	Acceptance string `targ:"flag,default=TBD,desc=Acceptance criteria"`
}

// DedupeArgs are arguments for the dedupe command.
type DedupeArgs struct {
	File string `targ:"flag,default=issues.md,desc=Issue file to update"`
}

// ListArgs are arguments for the list command.
type ListArgs struct {
	File   string `targ:"flag,default=issues.md,desc=Issue file to read"`
	Status string `targ:"flag,desc=Filter by status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
	Query  string `targ:"flag,desc=Case-insensitive title filter"`
	Source string `targ:"flag,default=all,desc=Issue source,enum=all|local|github"`
}

// MoveArgs are arguments for the move command.
type MoveArgs struct {
	File   string `targ:"flag,default=issues.md,desc=Issue file to update"`
	ID     string `targ:"positional,required,desc=Issue ID (e.g. 5 for local or gh#5 for GitHub)"`
	Status string `targ:"flag,required,desc=New status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
}

// UpdateArgs are arguments for the update command.
type UpdateArgs struct {
	File       string `targ:"flag,default=issues.md,desc=Issue file to update"`
	ID         string `targ:"positional,required,desc=Issue ID (e.g. 5 for local or gh#5 for GitHub)"`
	Status     string `targ:"flag,desc=New status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
	Desc       string `targ:"flag,name=description,desc=Description text"`
	Priority   string `targ:"flag,desc=Priority,enum=low|medium|high"`
	Acceptance string `targ:"flag,desc=Acceptance criteria"`
	Details    string `targ:"flag,desc=Implementation details"`
}

// ValidateArgs are arguments for the validate command.
type ValidateArgs struct {
	File string `targ:"flag,default=issues.md,desc=Issue file to validate"`
}

type issue struct {
	Number  int
	Title   string
	Section string
	Status  string
	Start   int
	End     int
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

type issueUpdates struct {
	Status      *string
	Description *string
	Priority    *string
	Acceptance  *string
	Details     *string
}

func create(c CreateArgs) error {
	if c.GitHub {
		num, err := createGitHubIssue(c.Title, c.Desc)
		if err != nil {
			return err
		}
		fmt.Printf("Created GitHub issue: %s\n", formatIssueID("github", num))
		return nil
	}

	filePath := findIssueFile(c.File)
	content, issues, err := loadIssues(filePath)
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
	return writeIssues(filePath, content.lines)
}

func dedupe(c DedupeArgs) error {
	filePath := findIssueFile(c.File)
	content, _, err := loadIssues(filePath)
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
	return writeIssues(filePath, file.lines)
}

// findIssueFile searches downward from the current directory for a file named
// issues.md. Returns the path if found, or the original path if not found.
func findIssueFile(path string) string {
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

func issueBlockLines(lines []string, iss issue) []string {
	block := make([]string, iss.End-iss.Start)
	copy(block, lines[iss.Start:iss.End])
	return block
}

func list(c ListArgs) error {
	type displayIssue struct {
		ID     string
		Status string
		Title  string
	}
	var all []displayIssue

	// Load local issues
	if c.Source == "all" || c.Source == "local" {
		file, issues, err := loadIssues(findIssueFile(c.File))
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

func move(c MoveArgs) error {
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
	filePath := findIssueFile(c.File)
	content, _, err := loadIssues(filePath)
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
	return writeIssues(filePath, file.lines)
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

func update(c UpdateArgs) error {
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
	filePath := findIssueFile(c.File)
	file, _, err := loadIssues(filePath)
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
	return writeIssues(filePath, file.lines)
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

func validate(c ValidateArgs) error {
	_, issues, err := loadIssues(findIssueFile(c.File))
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

func writeIssues(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
