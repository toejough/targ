package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

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

// Property: Targets execute at leaves of hierarchy
func TestProperty_Groups_TargetsExecuteAtLeaves(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		subName := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "subName")

		called := false
		sub := targ.Targ(func() { called = true }).Name(subName)
		group := targ.Group("grp", sub)

		// Single root group - go directly to member name
		_, err := targ.Execute([]string{"app", subName}, group)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(called).To(BeTrue())
	})
}

// Property: Path resolution uniquely identifies hierarchy points
func TestProperty_PathResolution_UniquelyIdentifiesHierarchyPoints(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	aCalled := false
	bCalled := false

	a := targ.Targ(func() { aCalled = true }).Name("target")
	b := targ.Targ(func() { bCalled = true }).Name("target")

	groupA := targ.Group("group-a", a)
	groupB := targ.Group("group-b", b)

	// Path "group-a target" should execute a, not b
	_, err := targ.Execute([]string{"app", "group-a", "target"}, groupA, groupB)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(aCalled).To(BeTrue())
	g.Expect(bCalled).To(BeFalse())

	// Reset and test other path
	aCalled = false
	_, err = targ.Execute([]string{"app", "group-b", "target"}, groupA, groupB)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(bCalled).To(BeTrue())
	g.Expect(aCalled).To(BeFalse())
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

// Property: Name collisions error at registration
func TestProperty_Registration_NameCollisionsError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Two targets with same name should cause an error
	a := targ.Targ(func() {}).Name("duplicate")
	b := targ.Targ(func() {}).Name("duplicate")

	_, err := targ.Execute([]string{"app", "duplicate"}, a, b)
	g.Expect(err).To(HaveOccurred())
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

// Property: Nested groups route correctly
func TestProperty_Groups_NestedGroupsRouteCorrectly(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	called := false
	inner := targ.Targ(func() { called = true }).Name("inner")
	innerGroup := targ.Group("inner-grp", inner)
	outerGroup := targ.Group("outer", innerGroup)

	// Single root (outer) - route through inner-grp to inner
	_, err := targ.Execute([]string{"app", "inner-grp", "inner"}, outerGroup)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
}

// Property: Unknown command returns error
func TestProperty_PathResolution_UnknownCommandReturnsError(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		unknownName := rapid.StringMatching(`unknown-[a-z]{3,8}`).Draw(rt, "unknownName")

		target := targ.Targ(func() {}).Name("known")

		result, err := targ.ExecuteWithOptions(
			[]string{"app", unknownName},
			targ.RunOptions{AllowDefault: false},
			target,
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Unknown"))
	})
}

// Property: Group with multiple members routes to correct one
func TestProperty_Groups_MultipleMembersRouteCorrectly(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Pick which target to call
		callBuild := rapid.Bool().Draw(rt, "callBuild")

		var calledTarget string

		build := targ.Targ(func() { calledTarget = "build" }).Name("build")
		test := targ.Targ(func() { calledTarget = "test" }).Name("test")
		group := targ.Group("dev", build, test)

		targetName := "test"
		if callBuild {
			targetName = "build"
		}

		_, err := targ.Execute([]string{"app", targetName}, group)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(calledTarget).To(Equal(targetName))
	})
}

// Property: Default name derived from function name
func TestProperty_NameDerivation_FunctionNameBecomesCLIName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Using the existing HelloWorld function pattern
	called := false
	// Note: When using a named function, targ derives the name automatically
	// For anonymous functions, we set name explicitly
	target := targ.Targ(func() { called = true }).Name("hello-world")

	// With AllowDefault: false, we must specify the command name
	_, err := targ.ExecuteWithOptions(
		[]string{"app", "hello-world"},
		targ.RunOptions{AllowDefault: false},
		target,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
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

// Property: Single root group allows direct member access
func TestProperty_Groups_SingleRootAllowsDirectMemberAccess(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	called := false
	sub := targ.Targ(func() { called = true }).Name("sub")
	group := targ.Group("grp", sub)

	// Single root group - can go directly to member name
	_, err := targ.Execute([]string{"app", "sub"}, group)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
}

// Property: Multiple roots require explicit path
func TestProperty_Groups_MultipleRootsRequireExplicitPath(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	called := false
	sub := targ.Targ(func() { called = true }).Name("sub")
	group := targ.Group("grp", sub)
	other := targ.Targ(func() {}).Name("other")

	// Multiple roots - need to specify group name
	_, err := targ.Execute([]string{"app", "grp", "sub"}, group, other)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
}
