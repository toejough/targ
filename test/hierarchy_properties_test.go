package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

// Property: Namespace nodes (groups) are not directly executable
func TestProperty_Groups_NamespaceNodesAreNotExecutable(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	sub := targ.Targ(func() {}).Name("sub")
	group := targ.Group("grp", sub)

	// When there's only a group, calling without subcommand shows help (no error)
	// but doesn't execute the group itself
	called := false
	subWithTrack := targ.Targ(func() { called = true }).Name("sub")
	groupWithTrack := targ.Group("grp", subWithTrack)

	_, err := targ.Execute([]string{"app"}, groupWithTrack)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeFalse()) // Group itself not executed

	_ = group // Used for documentation
}

// Property: Double-dash (^) resets to root
func TestProperty_PathResolution_CaretResetsToRoot(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	subCalled := 0
	rootCalled := 0

	sub := targ.Targ(func() { subCalled++ }).Name("sub")
	group := targ.Group("grp", sub)
	rootTarget := targ.Targ(func() { rootCalled++ }).Name("root-target")

	// Execute sub, then ^ to reset, then root-target
	_, err := targ.Execute([]string{"app", "grp", "sub", "^", "root-target"}, group, rootTarget)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(subCalled).To(Equal(1))
	g.Expect(rootCalled).To(Equal(1))
}

// Property: Multiple targets can be executed in sequence
func TestProperty_Execution_MultipleTargetsRunSequentially(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	order := make([]string, 0)

	a := targ.Targ(func() { order = append(order, "a") }).Name("a")
	b := targ.Targ(func() { order = append(order, "b") }).Name("b")

	_, err := targ.Execute([]string{"app", "a", "b"}, a, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(order).To(Equal([]string{"a", "b"}))
}

// Property: Glob patterns match multiple targets
func TestProperty_Glob_PatternMatchesMultipleTargets(t *testing.T) {
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
}
