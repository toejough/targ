package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/discover"
	"github.com/toejough/targ/internal/runner"
)

func TestModuleCacheKey_ReplaceTargetEdits(t *testing.T) {
	t.Parallel()

	t.Run("EditingReplaceTargetChangesKey", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")
		writeTestFile(g, dep, "dep.go", "package dep\n\nconst Timeout = 30\n")

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		before, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())

		writeTestFile(g, dep, "dep.go", "package dep\n\nconst Timeout = 600\n")

		after, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(after).NotTo(Equal(before),
			"issue #27: editing a local replace target must invalidate the cached binary")
	})

	t.Run("UnchangedInputsKeepKeyStable", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")
		writeTestFile(g, dep, "dep.go", "package dep\n")

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		first, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())

		second, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(second).To(Equal(first), "no edits, no invalidation — the cache must still hit")
	})
}

func TestModuleFiles_FollowsNestedSymlinkedDirectory(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Level 2: a symlinked directory INSIDE a hashed tree. WalkDir reports it as
	// a non-directory and never reads its contents.
	root := t.TempDir()
	target := filepath.Join(root, "target")
	outside := filepath.Join(root, "outside")
	g.Expect(os.MkdirAll(filepath.Join(target, "direct"), 0o755)).To(Succeed())
	g.Expect(os.MkdirAll(outside, 0o755)).To(Succeed())
	writeTestFile(g, filepath.Join(target, "direct"), "a.go", "package a\n")
	writeTestFile(g, outside, "b.go", "package b\n")
	g.Expect(os.Symlink(outside, filepath.Join(target, "linked"))).To(Succeed())

	files, err := runner.ExportCollectModuleFiles(target)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(pathsOf(files)).To(ContainElement(HaveSuffix("a.go")))
	g.Expect(pathsOf(files)).To(ContainElement(HaveSuffix("b.go")),
		"a symlinked subdirectory's files must be hashed too")
}

func TestModuleFiles_FollowsSymlinkedReplaceTarget(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Level 1: the replace target itself is a symlink — one checkout shared
	// across projects. os.Stat follows the link so the dir looks present, but
	// WalkDir does not descend, so without resolution nothing is hashed.
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	g.Expect(os.MkdirAll(filepath.Join(realDir, "pkg"), 0o755)).To(Succeed())
	writeTestFile(g, filepath.Join(realDir, "pkg"), "a.go", "package pkg\n")

	link := filepath.Join(root, "link")
	g.Expect(os.Symlink(realDir, link)).To(Succeed())

	files, err := runner.ExportCollectModuleFiles(link)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(pathsOf(files)).To(ContainElement(HaveSuffix("a.go")),
		"a symlinked tree's files must be hashed, or edits to them never invalidate the cache")
}

func TestModuleFiles_TerminatesOnSymlinkCycle(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Following symlinks without a cycle guard does not terminate. Three shapes:
	// self-loop, ancestor-loop, and mutual loop.
	root := t.TempDir()
	selfLoop := filepath.Join(root, "self")
	g.Expect(os.MkdirAll(selfLoop, 0o755)).To(Succeed())
	writeTestFile(g, selfLoop, "a.go", "package a\n")
	g.Expect(os.Symlink(selfLoop, filepath.Join(selfLoop, "loop"))).To(Succeed())

	deep := filepath.Join(root, "deep", "nested")
	g.Expect(os.MkdirAll(deep, 0o755)).To(Succeed())
	g.Expect(os.Symlink(filepath.Join(root, "deep"), filepath.Join(deep, "up"))).To(Succeed())

	mutualX := filepath.Join(root, "x")
	mutualY := filepath.Join(root, "y")

	g.Expect(os.MkdirAll(mutualX, 0o755)).To(Succeed())
	g.Expect(os.MkdirAll(mutualY, 0o755)).To(Succeed())
	g.Expect(os.Symlink(mutualY, filepath.Join(mutualX, "toy"))).To(Succeed())
	g.Expect(os.Symlink(mutualX, filepath.Join(mutualY, "tox"))).To(Succeed())

	// Each fixture gets a real .go file so the assertion proves the walk both
	// terminated and did useful work. A non-nil assertion alone would pass
	// vacuously on a fixture holding only directories and loops.
	for _, dir := range []string{selfLoop, deep, mutualX} {
		writeTestFile(g, dir, "found.go", "package p\n")
	}

	// Assert a bounded file count, not mere presence. Removing the visited set
	// does NOT hang: filepath.EvalSymlinks gives up at ~128 hops with
	// "EvalSymlinks: too many links", and this walker treats an EvalSymlinks
	// failure as unreachable-not-fatal, so a guard-less run still terminates
	// after 64-128x redundant walking and a presence-only assertion passes.
	// Only the count discriminates. Each want below is specific to its fixture's
	// shape — see the per-case notes.
	for _, tc := range []struct {
		dir  string
		want int
	}{
		{selfLoop, 2}, // this fixture writes a.go above, plus found.go below
		{deep, 2},     // `up` re-enters `nested` once directly and once via the parent before the guard catches it
		{mutualX, 1},  // y holds no .go files, and its link back to x is already visited
	} {
		files, err := runner.ExportCollectModuleFiles(tc.dir)
		g.Expect(err).NotTo(HaveOccurred(), "a symlink cycle must terminate, not error")
		g.Expect(pathsOf(files)).To(ContainElement(HaveSuffix("found.go")),
			"the walk must terminate AND still collect the tree's own files")
		g.Expect(files).To(HaveLen(tc.want),
			"a cycle must be visited once, not re-walked until EvalSymlinks' internal link limit stops it")
	}
}

func TestPrepareBootstrap_ReplaceTargetEdits(t *testing.T) {
	t.Parallel()

	t.Run("EditingReplaceTargetChangesKey", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")
		writeTestFile(g, dep, "dep.go", "package dep\n\nconst Timeout = 30\n")

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		before, err := runner.ExportPrepareBootstrap(consumer, consumer, "example.com/consumer")
		g.Expect(err).NotTo(HaveOccurred())

		writeTestFile(g, dep, "dep.go", "package dep\n\nconst Timeout = 600\n")

		after, err := runner.ExportPrepareBootstrap(consumer, consumer, "example.com/consumer")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(after).NotTo(Equal(before),
			"issue #27 (single-module/isolated bootstrap path): editing a local "+
				"replace target must invalidate the cached binary")
	})
}

func TestProperty_ReplaceTargetCacheKey(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(rt)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")

		name := rapid.StringMatching(`[a-z][a-z0-9]{0,8}\.go`).Draw(rt, "name")

		content1 := "package dep\n// " + rapid.StringMatching(`[ -~]{0,40}`).Draw(rt, "content1") + "\n"
		content2 := "package dep\n// " + rapid.StringMatching(`[ -~]{0,40}`).Draw(rt, "content2") + "\n"

		if content1 == content2 {
			content2 += "//\n"
		}

		writeTestFile(g, dep, name, content1)

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		key1, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("b"))
		g.Expect(err).NotTo(HaveOccurred())

		key1again, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("b"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(key1again).To(Equal(key1), "determinism: same tree, same key")

		writeTestFile(g, dep, name, content2)

		key2, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("b"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(key2).NotTo(Equal(key1), "sensitivity: any content edit changes the key")
	})

	t.Run("AddingFileToReplaceTargetChangesKey", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")
		writeTestFile(g, dep, "dep.go", "package dep\n")

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		before, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())

		writeTestFile(g, dep, "extra.go", "package dep\n\nconst Extra = true\n")

		after, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(after).NotTo(Equal(before),
			"adding a file to a replace target must invalidate the cached binary")
	})

	t.Run("RemovingFileFromReplaceTargetChangesKey", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")
		writeTestFile(g, dep, "dep.go", "package dep\n")
		writeTestFile(g, dep, "extra.go", "package dep\n\nconst Extra = true\n")

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		before, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(os.Remove(filepath.Join(dep, "extra.go"))).To(Succeed())

		after, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(after).NotTo(Equal(before),
			"removing a file from a replace target must invalidate the cached binary")
	})
}

func TestReplaceDirFiles_CollectsFilesystemReplaceTargets(t *testing.T) {
	t.Parallel()

	t.Run("AbsoluteReplaceTargetFilesIncluded", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")
		writeTestFile(g, dep, "dep.go", "package dep\n")
		writeTestFile(g, dep, "dep_test.go", "package dep\n")

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		files, err := runner.ExportCollectReplaceDirFiles(consumer)
		g.Expect(err).NotTo(HaveOccurred())

		paths := make([]string, 0, len(files))
		for _, f := range files {
			paths = append(paths, f.Path)
		}

		g.Expect(paths).To(ContainElement(filepath.Join(dep, "dep.go")))
		g.Expect(paths).To(ContainElement(filepath.Join(dep, "go.mod")))
		g.Expect(paths).NotTo(ContainElement(filepath.Join(dep, "dep_test.go")),
			"same walk as collectModuleFiles: test files excluded")
	})

	t.Run("RelativeReplaceTargetResolvedAgainstModuleRoot", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		root := t.TempDir()
		consumer := filepath.Join(root, "consumer")
		dep := filepath.Join(root, "dep")

		g.Expect(os.MkdirAll(consumer, 0o750)).To(Succeed())
		g.Expect(os.MkdirAll(dep, 0o750)).To(Succeed())
		writeTestFile(g, dep, "dep.go", "package dep\n")
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => ../dep\n")

		files, err := runner.ExportCollectReplaceDirFiles(consumer)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(files).To(HaveLen(1))
		g.Expect(files[0].Path).To(Equal(filepath.Join(dep, "dep.go")))
	})

	t.Run("ModuleVersionReplaceIgnored", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\n"+
				"require example.com/dep v1.0.0\n\n"+
				"replace example.com/dep => example.com/fork v1.0.1\n")

		files, err := runner.ExportCollectReplaceDirFiles(consumer)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(files).To(BeEmpty(), "module-version replaces are fingerprinted by go.sum already")
	})

	t.Run("MissingGoModYieldsNoFiles", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		files, err := runner.ExportCollectReplaceDirFiles(t.TempDir())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(files).To(BeEmpty())
	})

	t.Run("MissingReplaceTargetYieldsMarkerNotError", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		consumer := t.TempDir()
		gone := filepath.Join(consumer, "does-not-exist")
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+gone+"\n")

		files, err := runner.ExportCollectReplaceDirFiles(consumer)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(files).To(HaveLen(1))
		g.Expect(files[0].Path).To(HavePrefix("replace-missing:"),
			"existence flips must still change the cache key")
	})

	t.Run("UnparseableGoModDegradesToNoReplaceDirs", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod", "module \x00 not a go.mod")

		files, err := runner.ExportCollectReplaceDirFiles(consumer)
		g.Expect(err).NotTo(HaveOccurred(),
			"a future-toolchain go.mod must not hard-fail every targ run")
		g.Expect(files).To(BeEmpty())
	})
}

func TestReplaceDirFiles_FollowsSymlinkedTargetThroughGoMod(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// The production path: a real `replace … => <symlink>` directive, driven
	// through filesystemReplaceDirs and collectReplaceDirFiles. The walker-level
	// tests above call ExportCollectModuleFiles directly and never reach this
	// path, so only this test covers what a user's go.mod actually triggers.
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	g.Expect(os.MkdirAll(realDir, 0o755)).To(Succeed())
	writeTestFile(g, realDir, "dep.go", "package dep\n")

	link := filepath.Join(root, "link")
	g.Expect(os.Symlink(realDir, link)).To(Succeed())

	consumer := t.TempDir()
	writeTestFile(g, consumer, "go.mod", "module consumer\n\ngo 1.25.5\n\nreplace example.com/dep => "+link+"\n")

	files, err := runner.ExportCollectReplaceDirFiles(consumer)
	g.Expect(err).NotTo(HaveOccurred())

	// Assert the exact path, not just the suffix. A suffix match is blind to a
	// physical-vs-logical prefix mismatch (/private/var vs /var on macOS), so it
	// would still pass if the walker recorded resolved paths instead of the ones
	// go.mod names.
	g.Expect(pathsOf(files)).To(ContainElement(filepath.Join(link, "dep.go")),
		"a symlinked replace target must be hashed under the path go.mod names, not its physical location")
}

func TestReplaceDirFiles_SkipsModuleCacheTargets(t *testing.T) {
	g := NewWithT(t)

	// Release mode points the replace directive at targ's own directory inside
	// GOMODCACHE, which Go verifies by hash and never mutates in place — so
	// walking it detects a change that cannot happen. Not parallel: t.Setenv.
	modCache := t.TempDir()
	t.Setenv("GOMODCACHE", modCache)

	dep := filepath.Join(modCache, "example.com", "dep@v1.0.0")
	g.Expect(os.MkdirAll(dep, 0o755)).To(Succeed())
	writeTestFile(g, dep, "a.go", "package dep\n")

	consumer := t.TempDir()
	writeTestFile(g, consumer, "go.mod", "module consumer\n\ngo 1.25.5\n\nreplace example.com/dep => "+dep+"\n")

	files, err := runner.ExportCollectReplaceDirFiles(consumer)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(files).To(BeEmpty(),
		"module-cache targets are immutable; hashing them costs a full walk per invocation and buys nothing")
}

// pathsOf maps tagged files to their paths.
func pathsOf(files []discover.TaggedFile) []string {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}

	return paths
}

// writeTestFile writes content to dir/name, creating it if needed.
func writeTestFile(g *GomegaWithT, dir, name, content string) {
	g.THelper()
	g.Expect(os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)).To(Succeed())
}
