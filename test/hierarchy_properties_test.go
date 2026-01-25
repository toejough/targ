package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

//nolint:funlen // subtest container
func TestProperty_Glob(t *testing.T) {
	t.Parallel()

	t.Run("ContainsMatchWorks", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		testCalls := 0
		buildCalls := 0
		targetTest := targ.Targ(func() { testCalls++ }).Name("run-test-unit")
		targetBuild := targ.Targ(func() { buildCalls++ }).Name("build")

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "*test*"},
			targ.RunOptions{AllowDefault: false},
			targetTest, targetBuild,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(testCalls).To(Equal(1))
		g.Expect(buildCalls).To(Equal(0))
	})

	t.Run("NoMatchesReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build")
		test := targ.Targ(func() {}).Name("test")

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "nonexistent-*"},
			targ.RunOptions{AllowDefault: false, Overrides: targ.RuntimeOverrides{Parallel: true}},
			build, test,
		)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("PatternMatchesMultipleTargets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		testACalls := 0
		testBCalls := 0
		buildCalls := 0
		testA := targ.Targ(func() { testACalls++ }).Name("test-a")
		testB := targ.Targ(func() { testBCalls++ }).Name("test-b")
		build := targ.Targ(func() { buildCalls++ }).Name("build")

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "test-*"},
			targ.RunOptions{AllowDefault: false},
			testA, testB, build,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(testACalls).To(Equal(1))
		g.Expect(testBCalls).To(Equal(1))
		g.Expect(buildCalls).To(Equal(0))
	})

	t.Run("PatternMatchesSubcommands", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		testACalls := 0
		testBCalls := 0
		buildCalls := 0
		testA := targ.Targ(func() { testACalls++ }).Name("test-a")
		testB := targ.Targ(func() { testBCalls++ }).Name("test-b")
		build := targ.Targ(func() { buildCalls++ }).Name("build")
		group := targ.Group("dev", testA, testB, build)

		_, err := targ.Execute([]string{"app", "test-*"}, group)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(testACalls).To(Equal(1))
		g.Expect(testBCalls).To(Equal(1))
		g.Expect(buildCalls).To(Equal(0))
	})

	t.Run("StarMatchesAllTargets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		aCalls := 0
		bCalls := 0
		targetA := targ.Targ(func() { aCalls++ }).Name("alpha")
		targetB := targ.Targ(func() { bCalls++ }).Name("beta")

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "*"},
			targ.RunOptions{AllowDefault: false},
			targetA, targetB,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aCalls).To(Equal(1))
		g.Expect(bCalls).To(Equal(1))
	})

	t.Run("SuffixMatchWorks", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		unitCalls := 0
		intCalls := 0
		targetUnit := targ.Targ(func() { unitCalls++ }).Name("test-unit")
		targetInt := targ.Targ(func() { intCalls++ }).Name("test-int")

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "*-unit"},
			targ.RunOptions{AllowDefault: false},
			targetUnit, targetInt,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(unitCalls).To(Equal(1))
		g.Expect(intCalls).To(Equal(0))
	})
}

func TestProperty_Groups_NamespaceNodesAreNotExecutable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sub := targ.Targ(func() {}).Name("sub")
	group := targ.Group("grp", sub)

	called := false
	subWithTrack := targ.Targ(func() { called = true }).Name("sub")
	groupWithTrack := targ.Group("grp", subWithTrack)

	_, err := targ.Execute([]string{"app"}, groupWithTrack)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeFalse())

	_ = group
}

func TestProperty_Help_NestedGroupsShowChainExample(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	sub := targ.Targ(func() {}).Name("sub")
	group := targ.Group("grp", sub)
	other := targ.Targ(func() {}).Name("other")

	result, err := targ.Execute([]string{"app", "--help"}, group, other)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Output).To(ContainSubstring("grp"))
	g.Expect(result.Output).To(ContainSubstring("other"))
}

func TestProperty_PathResolution_CaretResetsToRoot(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	subCalled := 0
	rootCalled := 0
	sub := targ.Targ(func() { subCalled++ }).Name("sub")
	group := targ.Group("grp", sub)
	rootTarget := targ.Targ(func() { rootCalled++ }).Name("root-target")

	_, err := targ.Execute([]string{"app", "grp", "sub", "^", "root-target"}, group, rootTarget)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(subCalled).To(Equal(1))
	g.Expect(rootCalled).To(Equal(1))
}
