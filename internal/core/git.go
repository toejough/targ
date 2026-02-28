package core

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	internalsh "github.com/toejough/targ/internal/sh"
)

// CommandRunner executes a command and returns its combined output.
type CommandRunner func(ctx context.Context, name string, args ...string) (string, error)

// FileOpener is a function that opens a file for reading.
type FileOpener func(path string) (io.ReadCloser, error)

// CheckCleanWorkTree verifies the git working tree has no uncommitted changes.
func CheckCleanWorkTree(ctx context.Context) error {
	return CheckCleanWorkTreeWith(ctx, defaultCommandRunner)
}

// CheckCleanWorkTreeWith is a testable version with injected command runner.
// It checks for staged and unstaged changes to tracked files only (untracked files are ignored).
func CheckCleanWorkTreeWith(ctx context.Context, run CommandRunner) error {
	out, err := run(ctx, "git", "diff", "HEAD", "--stat")
	if err != nil {
		return fmt.Errorf("git diff: %w", err)
	}

	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("%w:\n%s", errUncommittedChanges, out)
	}

	return nil
}

// DetectRepoURL attempts to find the repository URL by parsing .git/config.
// It walks up from the current directory looking for a .git directory,
// then parses the config file for the remote "origin" URL.
// Returns empty string if not found.
func DetectRepoURL() string {
	return DetectRepoURLWithDeps(os.Getwd, osOpen)
}

// DetectRepoURLFromDirWithOpen walks up from dir looking for .git/config using injected opener.
func DetectRepoURLFromDirWithOpen(dir string, open FileOpener) string {
	for {
		gitConfig := filepath.Join(dir, ".git", "config")
		if url := ParseGitConfigOriginURLWithOpen(gitConfig, open); url != "" {
			return url
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}

		dir = parent
	}
}

// DetectRepoURLWithDeps is a testable version that accepts injected dependencies.
func DetectRepoURLWithDeps(getwd func() (string, error), open FileOpener) string {
	dir, err := getwd()
	if err != nil {
		return ""
	}

	return DetectRepoURLFromDirWithOpen(dir, open)
}

// NormalizeGitURL converts git@host:path to https://host/path format.
func NormalizeGitURL(url string) string {
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

// ParseGitConfigContent extracts the origin remote URL from git config content.
// This is a pure function that operates on an io.Reader.
func ParseGitConfigContent(r io.Reader) string {
	scanner := bufio.NewScanner(r)
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
				return NormalizeGitURL(strings.TrimSpace(parts[1]))
			}
		}
	}

	return ""
}

// ParseGitConfigOriginURLWithOpen reads a git config file using injected opener.
func ParseGitConfigOriginURLWithOpen(path string, open FileOpener) string {
	f, err := open(path)
	if err != nil {
		return ""
	}

	defer func() { _ = f.Close() }()

	return ParseGitConfigContent(f)
}

// unexported variables.
var (
	errUncommittedChanges = errors.New("uncommitted changes found")
)

// defaultCommandRunner wraps internalsh.OutputContext.
func defaultCommandRunner(ctx context.Context, name string, args ...string) (string, error) {
	return internalsh.OutputContext(ctx, name, args, os.Stdin)
}

// osOpen wraps os.Open to match the FileOpener signature.
func osOpen(path string) (io.ReadCloser, error) {
	f, err := os.Open(path) //nolint:gosec // path is .git/config, not user-controlled
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}

	return f, nil
}
