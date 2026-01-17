package core

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// DetectRepoURL attempts to find the repository URL by parsing .git/config.
// It walks up from the current directory looking for a .git directory,
// then parses the config file for the remote "origin" URL.
// Returns empty string if not found.
func DetectRepoURL() string {
	return detectRepoURLWithGetwd(os.Getwd)
}

// detectRepoURLFromDir walks up from dir looking for .git/config.
func detectRepoURLFromDir(dir string) string {
	for {
		gitConfig := filepath.Join(dir, ".git", "config")
		if url := parseGitConfigOriginURL(gitConfig); url != "" {
			return url
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}

		dir = parent
	}
}

// detectRepoURLWithGetwd is a testable version that accepts a working directory getter.
func detectRepoURLWithGetwd(getwd func() (string, error)) string {
	dir, err := getwd()
	if err != nil {
		return ""
	}

	return detectRepoURLFromDir(dir)
}

// normalizeGitURL converts git@host:path to https://host/path format.
func normalizeGitURL(url string) string {
	// Handle SSH format: git@github.com:user/repo.git
	if after, ok := strings.CutPrefix(url, "git@"); ok {
		url = after
		url = strings.Replace(url, ":", "/", 1)
		url = "https://" + url
	}

	// Remove .git suffix for cleaner URLs
	url = strings.TrimSuffix(url, ".git")

	return url
}

// parseGitConfigOriginURL reads a git config file and extracts the origin remote URL.
func parseGitConfigOriginURL(path string) string {
	f, err := os.Open(path) //nolint:gosec // path is .git/config, not user-controlled
	if err != nil {
		return ""
	}

	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	inOrigin := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Check for [remote "origin"] section
		if strings.HasPrefix(line, "[remote \"origin\"]") {
			inOrigin = true
			continue
		}

		// New section starts
		if strings.HasPrefix(line, "[") {
			inOrigin = false
			continue
		}

		// Look for url = ... in origin section
		if inOrigin && strings.HasPrefix(line, "url") {
			const keyValueParts = 2

			parts := strings.SplitN(line, "=", keyValueParts)
			if len(parts) == keyValueParts {
				return normalizeGitURL(strings.TrimSpace(parts[1]))
			}
		}
	}

	return ""
}
