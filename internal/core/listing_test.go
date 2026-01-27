package core

import (
	"bytes"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

func TestProperty_LocalTargetsShowLocal(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create a target with empty source
		targetName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "targetName")
		desc := rapid.String().Draw(t, "description")

		node := &commandNode{
			Name:        targetName,
			Description: desc,
			SourceFile:  "", // Empty source = local target
		}

		var buf bytes.Buffer

		width := len(targetName)

		// showAttribution = false for all-local targets (backwards compat)
		printTopLevelCommand(&buf, node, width, false)
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
		target := &Target{
			name:        targetName,
			description: desc,
			sourcePkg:   sourcePkg,
		}

		// Create commandNode
		node := &commandNode{
			Name:        targetName,
			Description: desc,
			Target:      target,
			SourceFile:  "/some/path/file.go", // Non-empty to indicate remote
		}

		var buf bytes.Buffer

		width := len(targetName)

		// showAttribution = true for remote targets
		printTopLevelCommand(&buf, node, width, true)
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
		target := &Target{
			name:           targetName,
			description:    desc,
			sourcePkg:      sourcePkg,
			nameOverridden: true, // This is the key - target was renamed
		}

		node := &commandNode{
			Name:        targetName,
			Description: desc,
			Target:      target,
			SourceFile:  "/some/path/file.go",
		}

		var buf bytes.Buffer

		width := len(targetName)

		// showAttribution = true for renamed remote targets
		printTopLevelCommand(&buf, node, width, true)
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

func TestProperty_NoSyncedTargetsUnchanged(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate multiple local targets
		numTargets := rapid.IntRange(1, 5).Draw(t, "numTargets")
		nodes := make([]*commandNode, numTargets)

		for i := range numTargets {
			targetName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "targetName")
			desc := rapid.String().Draw(t, "description")

			// All targets are local (no sourcePkg)
			nodes[i] = &commandNode{
				Name:        targetName,
				Description: desc,
				Target:      &Target{name: targetName, description: desc, sourcePkg: ""},
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
			printTopLevelCommand(&buf, node, width, false)
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

func TestProperty_MixedTargetsShowAttribution(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create a mix of local and remote targets
		localName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "localName")
		remoteName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "remoteName")

		// Ensure names are different
		if localName == remoteName {
			remoteName = remoteName + "-remote"
		}

		domain := rapid.StringMatching(`[a-z]+\.[a-z]+`).Draw(t, "domain")
		user := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "user")
		repo := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "repo")
		sourcePkg := domain + "/" + user + "/" + repo

		localNode := &commandNode{
			Name:        localName,
			Description: "local target",
			Target:      &Target{name: localName, sourcePkg: ""},
			SourceFile:  "",
		}

		remoteNode := &commandNode{
			Name:        remoteName,
			Description: "remote target",
			Target:      &Target{name: remoteName, sourcePkg: sourcePkg},
			SourceFile:  "/some/path.go",
		}

		var buf bytes.Buffer

		width := max(len(localName), len(remoteName))

		// showAttribution = true when we have mixed local and remote targets
		printTopLevelCommand(&buf, localNode, width, true)
		printTopLevelCommand(&buf, remoteNode, width, true)

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

func TestProperty_GroupsShowSourceAttribution(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create a mix with groups: local group, remote target
		localGroupName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "localGroupName")
		remoteTargetName := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "remoteTargetName")

		// Ensure names are different
		if localGroupName == remoteTargetName {
			remoteTargetName = remoteTargetName + "-target"
		}

		domain := rapid.StringMatching(`[a-z]+\.[a-z]+`).Draw(t, "domain")
		user := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "user")
		repo := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "repo")
		sourcePkg := domain + "/" + user + "/" + repo

		// Create a local group (no source)
		localGroup := &TargetGroup{
			name:    localGroupName,
			members: []any{},
		}

		// Create a remote target
		remoteTarget := &Target{
			name:      remoteTargetName,
			sourcePkg: sourcePkg,
		}

		// Parse both into commandNodes
		localGroupNode, err := parseGroupLike(localGroup)
		g.Expect(err).ToNot(HaveOccurred(), "parseGroupLike should succeed")

		remoteNode, err := parseTargetLike(remoteTarget)
		g.Expect(err).ToNot(HaveOccurred(), "parseTargetLike should succeed")

		var buf bytes.Buffer

		width := max(len(localGroupName), len(remoteTargetName))

		// showAttribution = true when we have mixed local and remote
		printTopLevelCommand(&buf, localGroupNode, width, true)
		printTopLevelCommand(&buf, remoteNode, width, true)

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
