package core

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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

// DetectRepoURL attempts to find the repository URL by parsing the repo's git
// config, walking up from the current directory. Detection is best-effort — it
// feeds optional help text — so any failure yields an empty string.
func DetectRepoURL() string {
	url, _ := DetectRepoURLWithDeps(os.Getwd, osOpen)

	return url
}

// DetectRepoURLFromDirWithOpen walks up from dir looking for a git config using
// the injected opener. A config that cannot be read stops the walk and returns
// an error, so a broken config never falls through to a parent repository's URL.
func DetectRepoURLFromDirWithOpen(dir string, open FileOpener) (string, error) {
	for {
		url, err := ParseGitConfigOriginURLWithOpen(filepath.Join(dir, ".git", "config"), open)
		if err != nil {
			return "", err
		}

		if url != "" {
			return url, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}

		dir = parent
	}
}

// DetectRepoURLWithDeps is a testable version that accepts injected dependencies.
// A getwd failure is wrapped and returned; otherwise detection proceeds as DetectRepoURLFromDirWithOpen.
func DetectRepoURLWithDeps(getwd func() (string, error), open FileOpener) (string, error) {
	dir, err := getwd()
	if err != nil {
		return "", fmt.Errorf("resolving working directory: %w", err)
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
// It returns an error only when the content could not be read to the end; a
// config with no origin section is ("", nil).
func ParseGitConfigContent(r io.Reader) (string, error) {
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
				return NormalizeGitURL(strings.TrimSpace(parts[1])), nil
			}
		}
	}

	err := scanner.Err()
	if err != nil {
		return "", fmt.Errorf("reading git config: %w", err)
	}

	return "", nil
}

// ParseGitConfigOriginURLWithOpen reads a git config file using injected opener.
// A file that is simply absent yields ("", nil) — the caller treats that as
// "no config here" and keeps walking. A config that exists but cannot be opened
// or read returns an error, so an unreadable repo never falls through to its
// parent.
func ParseGitConfigOriginURLWithOpen(path string, open FileOpener) (string, error) {
	f, err := open(path)
	if err != nil {
		// Absent, or the path ran through a .git that is a file rather than a
		// directory (a worktree pointer) — either way there is no config here,
		// so the caller keeps walking. Anything else means a config exists and
		// could not be read.
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return "", nil
		}

		return "", fmt.Errorf("opening git config: %w", err)
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
