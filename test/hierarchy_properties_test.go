// TEST-003: Hierarchy properties - validates group-based namespace and path traversal
// traces: ARCH-003, ARCH-004

//nolint:maintidx // Test functions with many subtests have low maintainability index by design
package targ_test

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

func TestProperty_Hierarchy(t *testing.T) {
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

		// Serial mode: glob with no matches
		_, err := targ.ExecuteWithOptions(
			[]string{"app", "nonexistent-*"},
			targ.RunOptions{AllowDefault: false},
			build, test,
		)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("NoMatchesParallelReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build")
		test := targ.Targ(func() {}).Name("test")

		// Parallel mode: glob with no matches (uses --parallel flag on CLI)
		_, err := targ.ExecuteWithOptions(
			[]string{"app", "--parallel", "nonexistent-*"},
			targ.RunOptions{AllowDefault: false},
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

	t.Run("StarMatchesSubcommands", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		aCalls := 0
		bCalls := 0
		a := targ.Targ(func() { aCalls++ }).Name("alpha")
		b := targ.Targ(func() { bCalls++ }).Name("beta")
		group := targ.Group("dev", a, b)
		other := targ.Targ(func() {}).Name("other")

		// Use dev/* to match all subcommands
		_, err := targ.Execute([]string{"app", "dev", "*"}, group, other)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aCalls).To(Equal(1))
		g.Expect(bCalls).To(Equal(1))
	})

	t.Run("ContainsMatchSubcommands", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		unitCalls := 0
		intCalls := 0
		buildCalls := 0
		unit := targ.Targ(func() { unitCalls++ }).Name("run-unit-tests")
		integration := targ.Targ(func() { intCalls++ }).Name("run-integration")
		build := targ.Targ(func() { buildCalls++ }).Name("build")
		group := targ.Group("dev", unit, integration, build)
		other := targ.Targ(func() {}).Name("other")

		// *unit* matches run-unit-tests (contains match)
		_, err := targ.Execute([]string{"app", "dev", "*unit*"}, group, other)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(unitCalls).To(Equal(1))
		g.Expect(intCalls).To(Equal(0))
		g.Expect(buildCalls).To(Equal(0))
	})

	t.Run("SuffixMatchSubcommands", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		unitCalls := 0
		intCalls := 0
		unit := targ.Targ(func() { unitCalls++ }).Name("test-unit")
		integration := targ.Targ(func() { intCalls++ }).Name("test-int")
		group := targ.Group("dev", unit, integration)
		other := targ.Targ(func() {}).Name("other")

		// *-unit matches test-unit (suffix match)
		_, err := targ.Execute([]string{"app", "dev", "*-unit"}, group, other)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(unitCalls).To(Equal(1))
		g.Expect(intCalls).To(Equal(0))
	})

	t.Run("DoubleStarMatchesSubcommands", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		aCalls := 0
		bCalls := 0
		a := targ.Targ(func() { aCalls++ }).Name("alpha")
		b := targ.Targ(func() { bCalls++ }).Name("beta")
		group := targ.Group("dev", a, b)
		other := targ.Targ(func() {}).Name("other")

		// ** matches all subcommands
		_, err := targ.Execute([]string{"app", "dev", "**"}, group, other)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aCalls).To(Equal(1))
		g.Expect(bCalls).To(Equal(1))
	})

	t.Run("ExactMatchSubcommands", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		aCalls := 0
		bCalls := 0
		a := targ.Targ(func() { aCalls++ }).Name("alpha")
		b := targ.Targ(func() { bCalls++ }).Name("beta")
		group := targ.Group("dev", a, b)
		other := targ.Targ(func() {}).Name("other")

		// Exact match (no wildcard)
		_, err := targ.Execute([]string{"app", "dev", "alpha"}, group, other)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aCalls).To(Equal(1))
		g.Expect(bCalls).To(Equal(0))
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

	t.Run("DoubleStarMatchesAllTargets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		aCalls := 0
		bCalls := 0
		targetA := targ.Targ(func() { aCalls++ }).Name("alpha")
		targetB := targ.Targ(func() { bCalls++ }).Name("beta")

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "**"},
			targ.RunOptions{AllowDefault: false},
			targetA, targetB,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aCalls).To(Equal(1))
		g.Expect(bCalls).To(Equal(1))
	})

	t.Run("DoubleStarWithSuffixGlobMatchesNestedTargets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		alphaCalls := 0
		betaCalls := 0
		alpha := targ.Targ(func() { alphaCalls++ }).Name("alpha")
		beta := targ.Targ(func() { betaCalls++ }).Name("beta")
		group := targ.Group("dev", alpha, beta)
		other := targ.Targ(func() {}).Name("other")

		// **/a* matches nested targets starting with "a"
		_, err := targ.ExecuteWithOptions(
			[]string{"app", "**/a*"},
			targ.RunOptions{AllowDefault: false},
			group, other,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(alphaCalls).To(Equal(1))
		g.Expect(betaCalls).To(Equal(0))
	})

	t.Run("ExactMatchWorks", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		buildCalls := 0
		testCalls := 0
		targetBuild := targ.Targ(func() { buildCalls++ }).Name("build")
		targetTest := targ.Targ(func() { testCalls++ }).Name("test")

		// Exact match without wildcards
		_, err := targ.ExecuteWithOptions(
			[]string{"app", "build"},
			targ.RunOptions{AllowDefault: false},
			targetBuild, targetTest,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(buildCalls).To(Equal(1))
		g.Expect(testCalls).To(Equal(0))
	})

	t.Run("ExactMatchIsCaseInsensitive", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		buildCalls := 0
		targetBuild := targ.Targ(func() { buildCalls++ }).Name("Build")
		targetTest := targ.Targ(func() {}).Name("test")

		// Case-insensitive exact match
		_, err := targ.ExecuteWithOptions(
			[]string{"app", "BUILD"},
			targ.RunOptions{AllowDefault: false},
			targetBuild, targetTest,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(buildCalls).To(Equal(1))
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

	t.Run("NamespaceNodesAreNotExecutable", func(t *testing.T) {
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
	})

	t.Run("NestedGroupsShowChainExample", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		sub := targ.Targ(func() {}).Name("sub")
		group := targ.Group("grp", sub)
		other := targ.Targ(func() {}).Name("other")

		result, err := targ.Execute([]string{"app", "--help"}, group, other)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("grp"))
		g.Expect(result.Output).To(ContainSubstring("other"))
	})

	t.Run("HelpWithCommandShowsCommandHelp", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type BuildArgs struct {
			Verbose bool `targ:"flag,desc=Enable verbose output"`
		}

		build := targ.Targ(func(_ BuildArgs) {}).Name("build").Description("Build the project")
		test := targ.Targ(func() {}).Name("test").Description("Run tests")

		// In multi-root mode, "build --help" should show build's specific help
		result, err := targ.Execute([]string{"app", "build", "--help"}, build, test)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("build"))
		g.Expect(result.Output).To(ContainSubstring("Build the project"))
		g.Expect(result.Output).To(ContainSubstring("--verbose"))
	})

	t.Run("HelpWithoutCommandShowsGlobalHelp", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build").Description("Build the project")
		test := targ.Targ(func() {}).Name("test").Description("Run tests")

		// In multi-root mode, "--help" without command shows all commands
		result, err := targ.Execute([]string{"app", "--help"}, build, test)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("build"))
		g.Expect(result.Output).To(ContainSubstring("test"))
	})

	t.Run("HelpShowsMoreInfoText", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("cmd")

		result, err := targ.ExecuteWithOptions(
			[]string{"app", "--help"},
			targ.RunOptions{MoreInfoText: "See https://example.com for docs"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("More info:"))
		g.Expect(result.Output).To(ContainSubstring("https://example.com"))
	})

	t.Run("HelpShowsSubcommandsWithoutDescriptions", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Subcommand without description (no .Description() call)
		subNoDesc := targ.Targ(func() {}).Name("no-desc")
		// Subcommand with description
		subWithDesc := targ.Targ(func() {}).Name("with-desc").Description("Has a description")
		group := targ.Group("grp", subNoDesc, subWithDesc)

		result, err := targ.Execute([]string{"app", "--help"}, group)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("no-desc"))
		g.Expect(result.Output).To(ContainSubstring("with-desc"))
		g.Expect(result.Output).To(ContainSubstring("Has a description"))
	})

	t.Run("HelpInDefaultModeShowsTargetHelp", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			File string `targ:"positional,desc=File to process"`
		}

		target := targ.Targ(func(_ Args) {}).Name("process").Description("Process a file")

		// In default mode (single target), --help shows that target's help
		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Process a file"))
		g.Expect(result.Output).To(ContainSubstring("file"))
	})

	t.Run("HelpWithUnknownCommandShowsGlobalHelp", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build")
		test := targ.Targ(func() {}).Name("test")

		// "unknown --help" - unknown doesn't match any root, shows global help
		// (help flag is extracted before command matching, so global help is shown)
		result, err := targ.Execute([]string{"app", "unknown", "--help"}, build, test)
		// With --help, global help is shown even if command is unknown
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("build"))
		g.Expect(result.Output).To(ContainSubstring("test"))
	})

	t.Run("HelpShowsDependencies", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		lint := targ.Targ(func() {}).Name("lint")
		build := targ.Targ(func() {}).Name("build").Deps(lint)

		// Help for a target with deps should show "Deps:"
		result, err := targ.Execute([]string{"app", "build", "--help"}, build, lint)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Deps:"))
		g.Expect(result.Output).To(ContainSubstring("lint"))
	})

	t.Run("HelpShowsDepsWithMultipleDependencies", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		lint := targ.Targ(func() {}).Name("lint")
		test := targ.Targ(func() {}).Name("test")
		build := targ.Targ(func() {}).Name("build").Deps(lint, test)

		// Help should show all deps
		result, err := targ.Execute([]string{"app", "build", "--help"}, build, lint, test)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Deps:"))
		g.Expect(result.Output).To(ContainSubstring("lint"))
		g.Expect(result.Output).To(ContainSubstring("test"))
	})

	t.Run("HelpShowsRetryInfo", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() error { return nil }).Name("flaky").Retry()

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Retry:"))
		g.Expect(result.Output).To(ContainSubstring("yes"))
	})

	t.Run("HelpShowsRetryWithBackoff", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() error { return nil }).
			Name("flaky").
			Retry().
			Backoff(100*time.Millisecond, 2.0)

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Retry:"))
		g.Expect(result.Output).To(ContainSubstring("backoff"))
	})

	t.Run("HelpShowsTimeout", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("slow").Timeout(5 * time.Minute)

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Timeout:"))
		g.Expect(result.Output).To(ContainSubstring("5m"))
	})

	t.Run("HelpShowsTimes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("repeat").Times(3)

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Times:"))
		g.Expect(result.Output).To(ContainSubstring("3"))
	})

	t.Run("CaretResetsToRoot", func(t *testing.T) {
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
	})

	t.Run("ListCommandReturnsTargetNames", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// __list is an internal command used by completion scripts
		target1 := targ.Targ(func() {}).Name("build")
		target2 := targ.Targ(func() {}).Name("test")

		result, err := targ.Execute([]string{"app", "__list"}, target1, target2)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("build"))
		g.Expect(result.Output).To(ContainSubstring("test"))
	})
}
