//go:build targ

// Package issues provides issue list tooling for targ.
package issues

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/toejough/targ/internal/issuefile"
)

type List struct {
	File   string `targ:"flag,default=issues.md,desc=Issue file to read"`
	Status string `targ:"flag,desc=Filter by status,enum=backlog|selected|in-progress|review|done|cancelled|blocked|open"`
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
	issue, _ := file.Find(number)
	if issue == nil {
		return fmt.Errorf("issue %d not found", number)
	}
	block := issuefile.IssueBlockLines(file.Lines, *issue)
	block = issuefile.UpdateStatus(block, status)
	file.Remove(*issue)

	section := "backlog"
	if strings.EqualFold(status, "done") {
		section = "done"
	}
	if err := file.Insert(section, block); err != nil {
		return err
	}

	fmt.Printf("Moved local issue %s to %s\n", formatIssueID("local", number), status)
	return writeIssues(c.File, file.Lines)
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
	issues := file.Issues
	done := map[int]bool{}
	for _, issue := range issues {
		if issue.Section == "done" {
			done[issue.Number] = true
		}
	}
	for i := len(issues) - 1; i >= 0; i-- {
		issue := issues[i]
		if issue.Section == "backlog" && done[issue.Number] {
			file.Remove(issue)
		}
	}
	return writeIssues(c.File, file.Lines)
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

	updates := issuefile.IssueUpdates{}
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

	if updates == (issuefile.IssueUpdates{}) {
		return fmt.Errorf("no updates provided")
	}

	if _, err := file.UpdateIssue(number, updates); err != nil {
		return err
	}
	fmt.Printf("Updated local issue: %s\n", formatIssueID("local", number))
	return writeIssues(c.File, file.Lines)
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
	if err := content.Insert(section, block); err != nil {
		return err
	}
	fmt.Printf("Created local issue: %s\n", formatIssueID("local", newID))
	return writeIssues(c.File, content.Lines)
}

func loadIssues(path string) (*issuefile.File, []issuefile.Issue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	file, err := issuefile.Parse(string(data))
	if err != nil {
		return nil, nil, err
	}
	return file, file.Issues, nil
}

func writeIssues(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
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
