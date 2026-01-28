package core_test

import (
	"bytes"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

func TestProperty_DeregisteredPackagesShownInHelp(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate package paths
		numPkgs := rapid.IntRange(1, 3).Draw(t, "numPkgs")
		pkgs := make([]string, 0, numPkgs)

		for i := range numPkgs {
			domain := rapid.StringMatching(`[a-z]+\.[a-z]+`).Draw(t, "domain")
			user := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "user")
			repo := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "repo")
			pkgs = append(pkgs, domain+"/"+user+"/"+repo)
			_ = i
		}

		opts := core.RunOptions{
			DeregisteredPackages: pkgs,
		}

		var buf bytes.Buffer
		core.PrintDeregisteredPackagesForTest(&buf, opts)
		output := buf.String()

		// Property: header is shown
		g.Expect(output).To(ContainSubstring("Deregistered packages"))

		// Property: all packages are listed
		for _, pkg := range pkgs {
			g.Expect(output).To(ContainSubstring(pkg),
				"output should contain deregistered package: %s", pkg)
		}
	})
}

func TestProperty_GroupsShowSourceAttribution(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create a mix with groups: local group, remote target
		localGroupName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "localGroupName")
		remoteTargetName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "remoteTargetName")

		// Ensure names are different
		if localGroupName == remoteTargetName {
			remoteTargetName += "-target"
		}

		domain := rapid.StringMatching(`[a-z]+\.[a-z]+`).Draw(t, "domain")
		user := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "user")
		repo := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "repo")
		sourcePkg := domain + "/" + user + "/" + repo

		// Create a local group (no source)
		localGroup := core.NewTargetGroupForTest(localGroupName, []any{})

		// Create a remote target
		remoteTarget := core.NewTargetForTest(remoteTargetName, "", sourcePkg, false)

		// Parse both into commandNodes
		localGroupNode, err := core.ParseGroupLikeForTest(localGroup)
		g.Expect(err).ToNot(HaveOccurred(), "parseGroupLike should succeed")

		remoteNode, err := core.ParseTargetLikeForTest(remoteTarget)
		g.Expect(err).ToNot(HaveOccurred(), "parseTargetLike should succeed")

		var buf bytes.Buffer

		width := max(len(localGroupName), len(remoteTargetName))

		// showAttribution = true when we have mixed local and remote
		core.PrintTopLevelCommandForTest(&buf, localGroupNode, width, true)
		core.PrintTopLevelCommandForTest(&buf, remoteNode, width, true)

		output := buf.String()

		// Groups should show (local) when mixed with remote targets
		g.Expect(output).To(ContainSubstring(localGroupName),
			"output should contain local group name")
		g.Expect(output).To(ContainSubstring(remoteTargetName),
			"output should contain remote target name")

		// The local group should show (local) when mixed
		g.Expect(output).To(ContainSubstring("(local)"),
			"local group should show (local) when mixed with remote")
		g.Expect(output).To(ContainSubstring(sourcePkg),
			"remote target should show source package")
	})
}

func TestProperty_HasRemoteTargets(t *testing.T) {
	t.Parallel()

	t.Run("ReturnsFalseForNilTargetNodes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Node without Target (e.g., plain group node)
		nodes := []*core.CommandNodeForTest{
			{Name: "grp", Target: nil},
		}
		g.Expect(core.HasRemoteTargetsForTest(nodes)).To(BeFalse(),
			"nodes without Target should not be considered remote")
	})

	t.Run("ReturnsFalseForEmptySource", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		nodes := []*core.CommandNodeForTest{
			{Name: "local", Target: core.NewTargetForTest("local", "", "", false)},
		}
		g.Expect(core.HasRemoteTargetsForTest(nodes)).To(BeFalse(),
			"local targets should not be considered remote")
	})

	t.Run("ReturnsTrueForRemoteSource", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		nodes := []*core.CommandNodeForTest{
			{
				Name:   "remote",
				Target: core.NewTargetForTest("remote", "", "github.com/foo/bar", false),
			},
		}
		g.Expect(core.HasRemoteTargetsForTest(nodes)).To(BeTrue(),
			"target with sourcePkg should be considered remote")
	})

	t.Run("ReturnsFalseForEmptySlice", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.HasRemoteTargetsForTest(nil)).To(BeFalse(),
			"nil slice should not have remote targets")
	})
}

func TestProperty_LocalTargetsShowLocal(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create a target with empty source
		targetName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "targetName")
		desc := rapid.String().Draw(t, "description")

		node := &core.CommandNodeForTest{
			Name:        targetName,
			Description: desc,
			SourceFile:  "", // Empty source = local target
		}

		var buf bytes.Buffer

		width := len(targetName)

		// showAttribution = false for all-local targets (backwards compat)
		core.PrintTopLevelCommandForTest(&buf, node, width, false)
		output := buf.String()

		// When all targets are local, we should NOT show (local)
		// This is for backwards compatibility
		g.Expect(output).To(ContainSubstring(targetName),
			"output should contain target name")

		if desc != "" {
			g.Expect(output).To(ContainSubstring(desc),
				"output should contain description")
		}

		g.Expect(output).ToNot(ContainSubstring("(local)"),
			"should not show (local) attribution when all targets are local")
		// Check that there's no source package path pattern in the output
		g.Expect(output).ToNot(MatchRegexp(`\([a-z]+\.[a-z]+/`),
			"should not show source package when all targets are local")
	})
}

func TestProperty_MixedTargetsShowAttribution(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create a mix of local and remote targets
		localName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "localName")
		remoteName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "remoteName")

		// Ensure names are different
		if localName == remoteName {
			remoteName += "-remote"
		}

		domain := rapid.StringMatching(`[a-z]+\.[a-z]+`).Draw(t, "domain")
		user := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "user")
		repo := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "repo")
		sourcePkg := domain + "/" + user + "/" + repo

		localNode := &core.CommandNodeForTest{
			Name:        localName,
			Description: "local target",
			Target:      core.NewTargetForTest(localName, "", "", false),
			SourceFile:  "",
		}

		remoteNode := &core.CommandNodeForTest{
			Name:        remoteName,
			Description: "remote target",
			Target:      core.NewTargetForTest(remoteName, "", sourcePkg, false),
			SourceFile:  "/some/path.go",
		}

		var buf bytes.Buffer

		width := max(len(localName), len(remoteName))

		// showAttribution = true when we have mixed local and remote targets
		core.PrintTopLevelCommandForTest(&buf, localNode, width, true)
		core.PrintTopLevelCommandForTest(&buf, remoteNode, width, true)

		output := buf.String()

		// When we have a mix, local targets should show (local)
		// Check both targets are present with correct attribution
		g.Expect(output).To(ContainSubstring(localName),
			"output should contain local target name")
		g.Expect(output).To(ContainSubstring(remoteName),
			"output should contain remote target name")

		// The output should have both (local) and the source package
		g.Expect(output).To(ContainSubstring("(local)"),
			"local target should show (local) when mixed with remote")
		g.Expect(output).To(ContainSubstring(sourcePkg),
			"remote target should show source package")
	})
}

func TestProperty_NoDeregisteredPackagesHidesSection(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	opts := core.RunOptions{
		DeregisteredPackages: nil,
	}

	var buf bytes.Buffer
	core.PrintDeregisteredPackagesForTest(&buf, opts)

	g.Expect(buf.String()).To(BeEmpty(),
		"should not print anything when no deregistered packages")
}

func TestProperty_NoSyncedTargetsUnchanged(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate multiple local targets
		numTargets := rapid.IntRange(1, 5).Draw(t, "numTargets")
		nodes := make([]*core.CommandNodeForTest, numTargets)

		for i := range numTargets {
			targetName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "targetName")
			desc := rapid.String().Draw(t, "description")

			// All targets are local (no sourcePkg)
			nodes[i] = &core.CommandNodeForTest{
				Name:        targetName,
				Description: desc,
				Target:      core.NewTargetForTest(targetName, desc, "", false),
				SourceFile:  "", // Empty source
			}
		}

		var buf bytes.Buffer

		width := 0

		for _, node := range nodes {
			if len(node.Name) > width {
				width = len(node.Name)
			}
		}

		// showAttribution = false when all targets are local (backwards compat)
		for _, node := range nodes {
			core.PrintTopLevelCommandForTest(&buf, node, width, false)
		}

		output := buf.String()

		// When ALL targets are local, we should NOT show (local) attribution
		// This maintains backwards compatibility
		g.Expect(output).ToNot(ContainSubstring("(local)"),
			"all-local listing should not show (local) annotation")
		// Check that there's no source package path pattern in the output
		g.Expect(output).ToNot(MatchRegexp(`\([a-z]+\.[a-z]+/`),
			"all-local listing should not have source package paths")
	})
}

func TestProperty_RemoteTargetsShowSource(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create target with source attribution
		targetName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "targetName")
		desc := rapid.String().Draw(t, "description")

		// Generate a source package path
		domain := rapid.StringMatching(`[a-z]+\.[a-z]+`).Draw(t, "domain")
		user := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "user")
		repo := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "repo")
		sourcePkg := domain + "/" + user + "/" + repo

		// Create a Target with source
		target := core.NewTargetForTest(targetName, desc, sourcePkg, false)

		// Create commandNode
		node := &core.CommandNodeForTest{
			Name:        targetName,
			Description: desc,
			Target:      target,
			SourceFile:  "/some/path/file.go", // Non-empty to indicate remote
		}

		var buf bytes.Buffer

		width := len(targetName)

		// showAttribution = true for remote targets
		core.PrintTopLevelCommandForTest(&buf, node, width, true)
		output := buf.String()

		g.Expect(output).To(ContainSubstring(targetName),
			"output should contain target name")

		if desc != "" {
			g.Expect(output).To(ContainSubstring(desc),
				"output should contain description")
		}

		g.Expect(output).To(ContainSubstring(sourcePkg),
			"output should contain source package: %q", sourcePkg)
	})
}

func TestProperty_RenamedTargetsAnnotated(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		targetName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "targetName")
		desc := rapid.String().Draw(t, "description")

		domain := rapid.StringMatching(`[a-z]+\.[a-z]+`).Draw(t, "domain")
		user := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "user")
		repo := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "repo")
		sourcePkg := domain + "/" + user + "/" + repo

		// Create a renamed target
		target := core.NewTargetForTest(targetName, desc, sourcePkg, true)

		node := &core.CommandNodeForTest{
			Name:        targetName,
			Description: desc,
			Target:      target,
			SourceFile:  "/some/path/file.go",
		}

		var buf bytes.Buffer

		width := len(targetName)

		// showAttribution = true for renamed remote targets
		core.PrintTopLevelCommandForTest(&buf, node, width, true)
		output := buf.String()

		g.Expect(output).To(ContainSubstring(targetName),
			"output should contain target name")

		if desc != "" {
			g.Expect(output).To(ContainSubstring(desc),
				"output should contain description")
		}

		g.Expect(output).To(ContainSubstring(sourcePkg),
			"output should contain source package")
		g.Expect(output).To(ContainSubstring("renamed"),
			"output should show renamed annotation")
	})
}
