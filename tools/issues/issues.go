//go:build commander

// Package issues provides issue list tooling for commander.
package issues

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"commander/internal/issuefile"
)

type List struct {
	File    string `commander:"flag,default=issues.md"`
	Status  string `commander:"flag"`
	Section string `commander:"flag"`
	Query   string `commander:"flag"`
}

func (c *List) Run() error {
	file, issues, err := loadIssues(c.File)
	if err != nil {
		return err
	}
	_ = file
	var filtered []issuefile.Issue
	for _, issue := range issues {
		if c.Section != "" && !strings.EqualFold(issue.Section, c.Section) {
			continue
		}
		if c.Status != "" && !strings.EqualFold(issue.Status, c.Status) {
			continue
		}
		if c.Query != "" && !strings.Contains(strings.ToLower(issue.Title), strings.ToLower(c.Query)) {
			continue
		}
		filtered = append(filtered, issue)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Number < filtered[j].Number
	})
	for _, issue := range filtered {
		fmt.Printf("%d\t%s\t%s\t%s\n", issue.Number, issue.Status, issue.Section, issue.Title)
	}
	return nil
}

type Move struct {
	File   string `commander:"flag,default=issues.md"`
	ID     int    `commander:"positional,required"`
	Status string `commander:"flag,required"`
}

func (c *Move) Run() error {
	content, _, err := loadIssues(c.File)
	if err != nil {
		return err
	}
	file := content
	issue, _ := file.Find(c.ID)
	if issue == nil {
		return fmt.Errorf("issue %d not found", c.ID)
	}
	block := issuefile.IssueBlockLines(file.Lines, *issue)
	block = issuefile.UpdateStatus(block, c.Status)
	file.Remove(*issue)

	section := "backlog"
	if strings.EqualFold(c.Status, "done") {
		section = "done"
	}
	if err := file.Insert(section, block); err != nil {
		return err
	}

	return writeIssues(c.File, file.Lines)
}

type Dedupe struct {
	File string `commander:"flag,default=issues.md"`
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
	File string `commander:"flag,default=issues.md"`
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
	File        string `commander:"flag,default=issues.md"`
	Title       string `commander:"flag,required"`
	Status      string `commander:"flag,default=backlog"`
	Description string `commander:"flag,default=TBD"`
	Priority    string `commander:"flag,default=Low"`
	Acceptance  string `commander:"flag,default=TBD"`
}

func (c *Create) Run() error {
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

	block := []string{
		fmt.Sprintf("### %d. %s", newID, c.Title),
		"",
		"#### Universal",
		"",
		"**Status**",
		strings.ToLower(c.Status),
		"",
		"**Description**",
		c.Description,
		"",
		"#### Planning",
		"",
		"**Priority**",
		c.Priority,
		"",
		"**Acceptance**",
		c.Acceptance,
	}

	section := "backlog"
	if strings.EqualFold(c.Status, "done") {
		section = "done"
	}
	if err := content.Insert(section, block); err != nil {
		return err
	}
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
