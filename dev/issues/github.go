//go:build targ

package issues

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/toejough/targ/sh"
)

// gitHubIssue represents an issue from GitHub.
type gitHubIssue struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	State  string   `json:"state"` // "open" or "closed"
	Body   string   `json:"body"`
	Labels []label  `json:"labels"`
}

// label represents a GitHub label.
type label struct {
	Name string `json:"name"`
}

// gitHubUpdates holds optional updates for a GitHub issue.
type gitHubUpdates struct {
	Title *string
	Body  *string
}

// listGitHubIssues fetches all issues from the current repo via gh CLI.
func listGitHubIssues(state string) ([]gitHubIssue, error) {
	args := []string{"issue", "list", "--json", "number,title,state,body,labels", "--limit", "100"}
	if state != "" && state != "all" {
		args = append(args, "--state", state)
	} else {
		args = append(args, "--state", "all")
	}
	out, err := sh.Output("gh", args...)
	if err != nil {
		return nil, fmt.Errorf("gh issue list failed: %w", err)
	}
	var issues []gitHubIssue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}
	return issues, nil
}

// createGitHubIssue creates a new issue on GitHub and returns its number.
func createGitHubIssue(title, body string) (int, error) {
	args := []string{"issue", "create", "--title", title}
	if body != "" {
		args = append(args, "--body", body)
	}
	out, err := sh.Output("gh", args...)
	if err != nil {
		return 0, fmt.Errorf("gh issue create failed: %w", err)
	}
	// Output is the issue URL, extract number from it
	// e.g., https://github.com/owner/repo/issues/123
	parts := strings.Split(strings.TrimSpace(out), "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("unexpected gh output: %s", out)
	}
	numStr := parts[len(parts)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("parsing issue number from %q: %w", out, err)
	}
	return num, nil
}

// updateGitHubIssue updates an existing GitHub issue.
func updateGitHubIssue(number int, updates gitHubUpdates) error {
	args := []string{"issue", "edit", strconv.Itoa(number)}
	if updates.Title != nil {
		args = append(args, "--title", *updates.Title)
	}
	if updates.Body != nil {
		args = append(args, "--body", *updates.Body)
	}
	if len(args) == 3 {
		return fmt.Errorf("no updates provided")
	}
	_, err := sh.Output("gh", args...)
	if err != nil {
		return fmt.Errorf("gh issue edit failed: %w", err)
	}
	return nil
}

// closeGitHubIssue closes a GitHub issue.
func closeGitHubIssue(number int) error {
	_, err := sh.Output("gh", "issue", "close", strconv.Itoa(number))
	if err != nil {
		return fmt.Errorf("gh issue close failed: %w", err)
	}
	return nil
}

// reopenGitHubIssue reopens a closed GitHub issue.
func reopenGitHubIssue(number int) error {
	_, err := sh.Output("gh", "issue", "reopen", strconv.Itoa(number))
	if err != nil {
		return fmt.Errorf("gh issue reopen failed: %w", err)
	}
	return nil
}

// parseIssueID parses an issue ID string and returns the source and number.
// Examples: "5" → ("local", 5), "gh#5" → ("github", 5), "gh5" → ("github", 5)
func parseIssueID(id string) (source string, number int, err error) {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "#")

	if strings.HasPrefix(id, "gh#") {
		numStr := strings.TrimPrefix(id, "gh#")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid GitHub issue ID: %s", id)
		}
		return "github", num, nil
	}
	if strings.HasPrefix(id, "gh") {
		numStr := strings.TrimPrefix(id, "gh")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid GitHub issue ID: %s", id)
		}
		return "github", num, nil
	}

	num, err := strconv.Atoi(id)
	if err != nil {
		return "", 0, fmt.Errorf("invalid issue ID: %s", id)
	}
	return "local", num, nil
}

// formatIssueID formats an issue ID with its source prefix.
func formatIssueID(source string, number int) string {
	if source == "github" {
		return fmt.Sprintf("gh#%d", number)
	}
	return fmt.Sprintf("#%d", number)
}

// ghStateToStatus maps GitHub state to local status.
func ghStateToStatus(state string) string {
	switch strings.ToLower(state) {
	case "closed":
		return "done"
	default:
		return "backlog"
	}
}
