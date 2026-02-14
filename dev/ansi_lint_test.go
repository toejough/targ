//go:build targ

package dev

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// TestLint_NoDirectANSICodesOutsideHelp ensures ANSI escape codes are only
// used in internal/help (where lipgloss styles are defined).
func TestLint_NoDirectANSICodesOutsideHelp(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Find repo root (where go.mod is)
	repoRoot, err := findRepoRoot()
	g.Expect(err).NotTo(HaveOccurred(), "failed to find repo root")

	// Pattern matches common ANSI escape sequences in source code
	// \x1b[ - CSI (Control Sequence Introducer) - hex representation
	// \033[ - octal representation
	// \e[ - bash-style escape (sometimes appears in docs/comments)
	ansiPattern := regexp.MustCompile(`\\x1b\[|\\033\[|\\e\[`)

	// Find all Go files
	var violations []string

	err = filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip internal/help (ANSI codes are expected there via lipgloss)
		if strings.Contains(path, "internal/help") {
			return nil
		}

		// Skip test files (they may contain assertions about ANSI codes)
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip vendor directories
		if strings.Contains(path, "vendor/") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(repoRoot, path)
		lines := strings.Split(string(content), "\n")

		for i, line := range lines {
			if ansiPattern.MatchString(line) {
				violations = append(violations,
					fmt.Sprintf("%s:%d: %s", relPath, i+1, strings.TrimSpace(line)))
			}
		}

		return nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(violations).To(BeEmpty(),
		"Found direct ANSI escape codes outside internal/help:\n%s\n"+
			"Use lipgloss styles from internal/help instead.",
		strings.Join(violations, "\n"))
}

// findRepoRoot walks up from current directory to find go.mod.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod in any parent directory")
		}

		dir = parent
	}
}
