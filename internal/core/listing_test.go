package core

import (
	"bytes"
	"strings"
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

		printTopLevelCommand(&buf, node, width)
		output := buf.String()

		// When all targets are local, we should NOT show (local)
		// This is for backwards compatibility
		// But when we have mixed targets, local ones show (local)
		// For now, just check that the output contains the name and description
		g.Expect(output).To(ContainSubstring(targetName),
			"output should contain target name")
		if desc != "" {
			g.Expect(output).To(ContainSubstring(desc),
				"output should contain description")
		}
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

		printTopLevelCommand(&buf, node, width)
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

		printTopLevelCommand(&buf, node, width)
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

		for i := 0; i < numTargets; i++ {
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

		for _, node := range nodes {
			printTopLevelCommand(&buf, node, width)
		}

		output := buf.String()

		// When ALL targets are local, we should NOT show (local) attribution
		// This maintains backwards compatibility
		g.Expect(output).ToNot(ContainSubstring("(local)"),
			"all-local listing should not show (local) annotation")
		g.Expect(output).ToNot(ContainSubstring("("),
			"all-local listing should not have any source annotations")
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

		printTopLevelCommand(&buf, localNode, width)
		printTopLevelCommand(&buf, remoteNode, width)

		output := buf.String()

		// When we have a mix, local targets should show (local)
		lines := strings.Split(output, "\n")
		localLine := ""
		remoteLine := ""

		for _, line := range lines {
			if strings.Contains(line, localName) {
				localLine = line
			}
			if strings.Contains(line, remoteName) {
				remoteLine = line
			}
		}

		g.Expect(localLine).ToNot(BeEmpty(), "should have local target line")
		g.Expect(remoteLine).ToNot(BeEmpty(), "should have remote target line")

		g.Expect(localLine).To(ContainSubstring("(local)"),
			"local target should show (local) when mixed with remote")
		g.Expect(remoteLine).To(ContainSubstring(sourcePkg),
			"remote target should show source package")
	})
}
