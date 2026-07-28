//go:build integration

package core_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

// TestIntegrationCheckCleanWorkTreeIgnoresUntrackedFiles verifies the claim in
// the process-execution spec's "Only untracked files are present" scenario
// against the real git binary: `git diff HEAD --stat` produces no output for
// untracked files, so CheckCleanWorkTreeWith must report a clean tree. The
// unit-level mock test for this function only pins the general "non-empty
// output errors" rule; it cannot verify what real git actually emits for
// untracked files, so this runs the genuine subprocess against a scratch repo.
func TestIntegrationCheckCleanWorkTreeIgnoresUntrackedFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	dir := t.TempDir()
	runGit(t, dir, "init")
	g.Expect(os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("v1\n"), 0o600)).To(Succeed())
	runGit(t, dir, "add", "tracked.txt")
	runGit(t, dir, "commit", "-m", "initial commit")

	// An untracked file only — no staged or unstaged changes to a tracked file.
	g.Expect(os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new\n"), 0o600)).To(Succeed())

	runner := func(ctx context.Context, name string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir

		out, err := cmd.CombinedOutput()

		return string(out), err
	}

	err := core.CheckCleanWorkTreeWith(context.Background(), runner)
	g.Expect(err).NotTo(HaveOccurred(), "untracked files must not count as uncommitted changes")
}

// runGit runs git in dir with a hermetic author/committer identity, so the
// test never depends on (or mutates) the invoking machine's git config.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=targ-test",
		"GIT_AUTHOR_EMAIL=targ-test@example.com",
		"GIT_COMMITTER_NAME=targ-test",
		"GIT_COMMITTER_EMAIL=targ-test@example.com",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
