package core_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestProperty_CleanWorkTree(t *testing.T) {
	t.Parallel()

	t.Run("DefaultRunnerExecutesGit", func(t *testing.T) {
		t.Parallel()

		// CheckCleanWorkTree uses the real git command runner.
		// In a git repo it should succeed or fail — either way, no panic.
		_ = core.CheckCleanWorkTree(context.Background())
	})

	t.Run("CleanTreeReturnsNil", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		runner := func(_ context.Context, _ string, _ ...string) (string, error) {
			return "", nil
		}

		err := core.CheckCleanWorkTreeWith(context.Background(), runner)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("ModifiedFilesReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		runner := func(_ context.Context, _ string, _ ...string) (string, error) {
			return " M file.go", nil
		}

		err := core.CheckCleanWorkTreeWith(context.Background(), runner)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("uncommitted"))
	})

	t.Run("UntrackedFilesReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		runner := func(_ context.Context, _ string, _ ...string) (string, error) {
			return "?? new.go", nil
		}

		err := core.CheckCleanWorkTreeWith(context.Background(), runner)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("uncommitted"))
	})

	t.Run("StagedFilesReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		runner := func(_ context.Context, _ string, _ ...string) (string, error) {
			return "A  staged.go", nil
		}

		err := core.CheckCleanWorkTreeWith(context.Background(), runner)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("uncommitted"))
	})

	t.Run("GitCommandFailureReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		runner := func(_ context.Context, _ string, _ ...string) (string, error) {
			return "", errors.New("git not found")
		}

		err := core.CheckCleanWorkTreeWith(context.Background(), runner)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("git diff"))
	})

	t.Run("WhitespaceOnlyOutputReturnsNil", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		runner := func(_ context.Context, _ string, _ ...string) (string, error) {
			return "  \n  \t  ", nil
		}

		err := core.CheckCleanWorkTreeWith(context.Background(), runner)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("CorrectArgsPassed", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var capturedName string

		var capturedArgs []string

		runner := func(_ context.Context, name string, args ...string) (string, error) {
			capturedName = name
			capturedArgs = args

			return "", nil
		}

		err := core.CheckCleanWorkTreeWith(context.Background(), runner)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(capturedName).To(Equal("git"))
		g.Expect(capturedArgs).To(Equal([]string{"diff", "HEAD", "--stat"}))
	})
}

func TestProperty_GitDetection(t *testing.T) {
	t.Parallel()

	t.Run("ParseGitConfigContentReportsScanError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// A line longer than bufio.Scanner's 64KB default token cap aborts the
		// scan with bufio.ErrTooLong — the realistic read failure for a config.
		huge := "[remote \"origin\"]\n\turl = " + strings.Repeat("x", 70*1024) + "\n"

		url, err := core.ParseGitConfigContent(strings.NewReader(huge))
		g.Expect(err).To(HaveOccurred(), "an aborted scan must not look like a clean parse")
		g.Expect(url).To(BeEmpty())
	})

	t.Run("WalkUpStopsOnScanErrorInsteadOfUsingParentRepo", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// parent/ is a repo with a valid origin; parent/child/ is a repo whose
		// config cannot be scanned. Detection from child must NOT report the
		// parent's URL.
		parent := t.TempDir()
		g.Expect(os.MkdirAll(filepath.Join(parent, ".git"), 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(parent, ".git", "config"),
			[]byte("[remote \"origin\"]\n\turl = git@github.com:someone/parent.git\n"), 0o600)).To(Succeed())

		child := filepath.Join(parent, "child")
		g.Expect(os.MkdirAll(filepath.Join(child, ".git"), 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(child, ".git", "config"),
			[]byte("[remote \"origin\"]\n\turl = "+strings.Repeat("x", 70*1024)+"\n"), 0o600)).To(Succeed())

		url, err := core.DetectRepoURLFromDirWithOpen(child, core.OSOpenForTest)
		g.Expect(err).To(HaveOccurred(), "a broken config must stop the walk, not fall through to the parent")
		g.Expect(url).NotTo(ContainSubstring("parent"), "must never report a different repository's URL")
	})

	t.Run("DetectRepoURLReturnsRepoFromGitConfig", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// DetectRepoURL uses os.Getwd and os.Open, so in this repo it should find the URL
		url := core.DetectRepoURL()
		// In the targ repo, we expect a GitHub URL
		g.Expect(url).To(ContainSubstring("github.com"))
		g.Expect(url).To(ContainSubstring("targ"))
	})

	t.Run("ParseGitConfigContentExtractsOriginURL", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		config := `[core]
	repositoryformatversion = 0
	filemode = true
[remote "origin"]
	url = git@github.com:user/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
[branch "main"]
	remote = origin
`
		reader := strings.NewReader(config)
		url, err := core.ParseGitConfigContent(reader)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(url).To(Equal("https://github.com/user/repo"))
	})

	t.Run("ParseGitConfigContentHandlesMissingOrigin", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		config := `[core]
	repositoryformatversion = 0
[branch "main"]
	remote = origin
`
		reader := strings.NewReader(config)
		url, err := core.ParseGitConfigContent(reader)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(url).To(BeEmpty())
	})

	t.Run("NormalizeGitURLConvertsSSHToHTTPS", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		url := core.NormalizeGitURL("git@github.com:user/repo.git")
		g.Expect(url).To(Equal("https://github.com/user/repo"))
	})

	t.Run("NormalizeGitURLRemovesGitSuffix", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		url := core.NormalizeGitURL("https://github.com/user/repo.git")
		g.Expect(url).To(Equal("https://github.com/user/repo"))
	})

	t.Run("DetectRepoURLWithDepsHandlesGetwdError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		failingGetwd := func() (string, error) {
			return "", errors.New("getwd failed")
		}
		dummyOpen := func(_ string) (io.ReadCloser, error) {
			return nil, errors.New("should not be called")
		}

		url, err := core.DetectRepoURLWithDeps(failingGetwd, dummyOpen)
		g.Expect(err).To(HaveOccurred())
		g.Expect(url).To(BeEmpty())
	})

	t.Run("DetectRepoURLFromDirWithOpenHandlesOpenError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		failingOpen := func(_ string) (io.ReadCloser, error) {
			return nil, fs.ErrNotExist
		}

		// When open fails for all paths, eventually we reach root and return empty
		url, err := core.DetectRepoURLFromDirWithOpen("/tmp/nonexistent", failingOpen)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(url).To(BeEmpty())
	})

	t.Run("ParseGitConfigOriginURLWithOpenHandlesOpenError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		failingOpen := func(_ string) (io.ReadCloser, error) {
			return nil, fs.ErrNotExist
		}

		url, err := core.ParseGitConfigOriginURLWithOpen("/nonexistent/.git/config", failingOpen)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(url).To(BeEmpty())
	})
}
