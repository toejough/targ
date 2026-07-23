package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

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

// writeTestFile writes content to dir/name, creating it if needed.
func writeTestFile(g *GomegaWithT, dir, name, content string) {
	g.THelper()
	g.Expect(os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)).To(Succeed())
}
