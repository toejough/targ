//go:build targ

package internal

import (
	"fmt"
	"strings"
)

type CreateArgs struct {
	File       string `targ:"flag,default=issues.md,desc=Issue file to update"`
	GitHub     bool   `targ:"flag,desc=Create issue on GitHub instead of locally"`
	Title      string `targ:"flag,required,desc=Issue title"`
	Status     string `targ:"flag,default=backlog,desc=Initial status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
	Desc       string `targ:"flag,name=description,default=TBD,desc=Issue description"`
	Priority   string `targ:"flag,default=Low,desc=Priority,enum=low|medium|high"`
	Acceptance string `targ:"flag,default=TBD,desc=Acceptance criteria"`
}

type DedupeArgs struct {
	File string `targ:"flag,default=issues.md,desc=Issue file to update"`
}

type ListArgs struct {
	File   string `targ:"flag,default=issues.md,desc=Issue file to read"`
	Status string `targ:"flag,desc=Filter by status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
	Query  string `targ:"flag,desc=Case-insensitive title filter"`
	Source string `targ:"flag,default=all,desc=Issue source,enum=all|local|github"`
}

type MoveArgs struct {
	File   string `targ:"flag,default=issues.md,desc=Issue file to update"`
	ID     string `targ:"positional,required,desc=Issue ID (e.g. 5 for local or gh#5 for GitHub)"`
	Status string `targ:"flag,required,desc=New status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
}

type UpdateArgs struct {
	File       string `targ:"flag,default=issues.md,desc=Issue file to update"`
	ID         string `targ:"positional,required,desc=Issue ID (e.g. 5 for local or gh#5 for GitHub)"`
	Status     string `targ:"flag,desc=New status,enum=backlog|selected|in-progress|review|done|cancelled|blocked"`
	Desc       string `targ:"flag,name=description,desc=Description text"`
	Priority   string `targ:"flag,desc=Priority,enum=low|medium|high"`
	Acceptance string `targ:"flag,desc=Acceptance criteria"`
	Details    string `targ:"flag,desc=Implementation details"`
}

type ValidateArgs struct {
	File string `targ:"flag,default=issues.md,desc=Issue file to validate"`
}

func Create(c CreateArgs) error {
	if c.GitHub {
		num, err := CreateGitHubIssue(c.Title, c.Desc)
		if err != nil {
			return err
		}
		fmt.Printf("Created GitHub issue: %s\n", FormatIssueID("github", num))
		return nil
	}

	filePath := FindIssueFile(c.File)
	content, issues, err := LoadIssues(filePath)
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

	status := NormalizeStatus(c.Status)
	priority := NormalizePriority(c.Priority)
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
	fmt.Printf("Created local issue: %s\n", FormatIssueID("local", newID))
	return WriteIssues(filePath, content.Lines())
}

func Dedupe(c DedupeArgs) error {
	filePath := FindIssueFile(c.File)
	content, _, err := LoadIssues(filePath)
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
			file.Remove(iss)
		}
	}
	return WriteIssues(filePath, file.Lines())
}

func List(c ListArgs) error {
	var all []displayIssue

	// Load local issues
	if c.Source == "all" || c.Source == "local" {
		file, issues, err := LoadIssues(FindIssueFile(c.File))
		if err != nil && c.Source == "local" {
			return err
		}
		_ = file
		for _, issue := range issues {
			all = append(all, displayIssue{
				ID:     FormatIssueID("local", issue.Number),
				Status: NormalizeStatus(issue.Status),
				Title:  issue.Title,
			})
		}
	}

	// Load GitHub issues
	if c.Source == "all" || c.Source == "github" {
		ghIssues, err := ListGitHubIssues("")
		if err != nil && c.Source == "github" {
			return err
		}
		if err == nil {
			for _, issue := range ghIssues {
				all = append(all, displayIssue{
					ID:     FormatIssueID("github", issue.Number),
					Status: GhStateToStatus(issue.State),
					Title:  issue.Title,
				})
			}
		}
	}

	// Filter by status
	wantStatus := NormalizeStatus(c.Status)
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

func Move(c MoveArgs) error {
	source, number, err := ParseIssueID(c.ID)
	if err != nil {
		return err
	}

	status := NormalizeStatus(c.Status)

	if source == "github" {
		// For GitHub, moving to done/cancelled means closing, otherwise reopening
		if status == "done" || status == "cancelled" {
			if err := CloseGitHubIssue(number); err != nil {
				return err
			}
		} else {
			if err := ReopenGitHubIssue(number); err != nil {
				return err
			}
		}
		fmt.Printf("Moved GitHub issue %s to %s\n", FormatIssueID("github", number), status)
		return nil
	}

	// Handle local issue
	filePath := FindIssueFile(c.File)
	content, _, err := LoadIssues(filePath)
	if err != nil {
		return err
	}
	file := content
	iss, _ := file.Find(number)
	if iss == nil {
		return fmt.Errorf("issue %d not found", number)
	}
	block := IssueBlockLines(file.lines, *iss)
	block = UpdateIssueStatus(block, status)
	file.Remove(*iss)

	section := "backlog"
	if strings.EqualFold(status, "done") {
		section = "done"
	}
	if err := file.Insert(section, block); err != nil {
		return err
	}

	fmt.Printf("Moved local issue %s to %s\n", FormatIssueID("local", number), status)
	return WriteIssues(filePath, file.Lines())
}

func Update(c UpdateArgs) error {
	source, number, err := ParseIssueID(c.ID)
	if err != nil {
		return err
	}

	if source == "github" {
		// Handle GitHub issue update
		ghUpdates := GitHubUpdates{}
		if c.Desc != "" {
			ghUpdates.Body = &c.Desc
		}
		// GitHub doesn't have priority/acceptance/details fields directly
		// For status changes, we close/reopen
		if c.Status != "" {
			status := NormalizeStatus(c.Status)
			if status == "done" || status == "cancelled" {
				if err := CloseGitHubIssue(number); err != nil {
					return err
				}
			} else {
				if err := ReopenGitHubIssue(number); err != nil {
					return err
				}
			}
		}
		if ghUpdates.Body != nil {
			if err := UpdateGitHubIssue(number, ghUpdates); err != nil {
				return err
			}
		}
		fmt.Printf("Updated GitHub issue: %s\n", FormatIssueID("github", number))
		return nil
	}

	// Handle local issue update
	filePath := FindIssueFile(c.File)
	file, _, err := LoadIssues(filePath)
	if err != nil {
		return err
	}

	updates := IssueUpdates{}
	if c.Status != "" {
		status := NormalizeStatus(c.Status)
		updates.Status = &status
	}
	if c.Desc != "" {
		updates.Description = &c.Desc
	}
	if c.Priority != "" {
		priority := NormalizePriority(c.Priority)
		updates.Priority = &priority
	}
	if c.Acceptance != "" {
		updates.Acceptance = &c.Acceptance
	}
	if c.Details != "" {
		updates.Details = &c.Details
	}

	if updates == (IssueUpdates{}) {
		return fmt.Errorf("no updates provided")
	}

	if _, err := file.UpdateIssue(number, updates); err != nil {
		return err
	}
	fmt.Printf("Updated local issue: %s\n", FormatIssueID("local", number))
	return WriteIssues(filePath, file.Lines())
}

func Validate(c ValidateArgs) error {
	_, issues, err := LoadIssues(FindIssueFile(c.File))
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
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

type displayIssue struct {
	ID     string
	Status string
	Title  string
}
