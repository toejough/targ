package runner_test

import (
	"bytes"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/runner"
)

func TestProperty_SupersedingDetection(t *testing.T) {
	t.Parallel()

	t.Run("MostLocalCommandWinsDispatch", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		registry := []runner.ExportModuleRegistry{
			{
				BinaryPath: "/local/bin",
				ModuleRoot: "/project",
				ModulePath: "project.local",
				Commands:   []runner.ExportCommandInfo{{Name: "build", Description: "local build"}},
			},
			{
				BinaryPath: "/ancestor/bin",
				ModuleRoot: "/home",
				ModulePath: "home.local",
				Commands:   []runner.ExportCommandInfo{{Name: "build", Description: "ancestor build"}},
			},
		}

		binary, found := runner.ExportFindCommandBinary(registry, "build")
		g.Expect(found).To(BeTrue())
		g.Expect(binary).To(Equal("/local/bin"), "most-local registry should win dispatch")
	})

	t.Run("CollectAnnotatesDuplicatesWithSuperseding", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		registry := []runner.ExportModuleRegistry{
			{
				BinaryPath: "/local/bin",
				ModuleRoot: "/project",
				ModulePath: "project.local",
				Commands: []runner.ExportCommandInfo{
					{Name: "build", Description: "local build"},
					{Name: "test", Description: "local test"},
				},
			},
			{
				BinaryPath: "/ancestor/bin",
				ModuleRoot: "/home",
				ModulePath: "home.local",
				Commands: []runner.ExportCommandInfo{
					{Name: "build", Description: "ancestor build"},
					{Name: "deploy", Description: "ancestor deploy"},
				},
			},
		}

		cmds := runner.ExportCollectSortedCommands(registry)

		var localBuild, ancestorBuild *runner.ExportCmdEntry

		for i := range cmds {
			if cmds[i].Name == "build" {
				if cmds[i].SupersededBy == "" {
					localBuild = &cmds[i]
				} else {
					ancestorBuild = &cmds[i]
				}
			}
		}

		g.Expect(localBuild).NotTo(BeNil(), "primary build should exist")
		g.Expect(localBuild.Supersedes).To(ContainSubstring("/home"), "primary should note what it supersedes")

		g.Expect(ancestorBuild).NotTo(BeNil(), "superseded build should exist")
		g.Expect(ancestorBuild.SupersededBy).To(ContainSubstring("/project"), "superseded should note what supersedes it")
	})

	t.Run("RegistryOrderDeterminesLocality", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		registry := []runner.ExportModuleRegistry{
			{
				BinaryPath: "/ancestor/bin",
				ModuleRoot: "/home",
				ModulePath: "home.local",
				Commands:   []runner.ExportCommandInfo{{Name: "build", Description: "ancestor build"}},
			},
			{
				BinaryPath: "/local/bin",
				ModuleRoot: "/project",
				ModulePath: "project.local",
				Commands:   []runner.ExportCommandInfo{{Name: "build", Description: "local build"}},
			},
		}

		binary, found := runner.ExportFindCommandBinary(registry, "build")
		g.Expect(found).To(BeTrue())
		g.Expect(binary).To(Equal("/ancestor/bin"), "first registry entry should win")

		cmds := runner.ExportCollectSortedCommands(registry)

		for _, cmd := range cmds {
			if cmd.Name == "build" && cmd.SupersededBy == "" {
				g.Expect(cmd.Source).To(Equal("/home"), "primary should be from first registry")
			}
		}
	})

	t.Run("NonDuplicateCommandsUnaffected", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		registry := []runner.ExportModuleRegistry{
			{
				BinaryPath: "/local/bin",
				ModuleRoot: "/project",
				ModulePath: "project.local",
				Commands:   []runner.ExportCommandInfo{{Name: "build", Description: "build stuff"}},
			},
			{
				BinaryPath: "/ancestor/bin",
				ModuleRoot: "/home",
				ModulePath: "home.local",
				Commands:   []runner.ExportCommandInfo{{Name: "deploy", Description: "deploy stuff"}},
			},
		}

		cmds := runner.ExportCollectSortedCommands(registry)

		for _, cmd := range cmds {
			g.Expect(cmd.SupersededBy).To(BeEmpty(), "command %s should not be superseded", cmd.Name)
			g.Expect(cmd.Supersedes).To(BeEmpty(), "command %s should not supersede", cmd.Name)
		}
	})
}

func TestProperty_SupersedingDisplay(t *testing.T) {
	t.Parallel()

	t.Run("SupersededCommandsShowAnnotation", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		cmds := []runner.ExportCmdEntry{
			{Name: "build", Description: "local build", Source: "/project", Supersedes: "/home"},
			{Name: "build", Description: "ancestor build", Source: "/home", SupersededBy: "/project"},
			{Name: "deploy", Description: "deploy stuff", Source: "/home"},
		}

		var buf bytes.Buffer
		runner.ExportWriteCommandList(&buf, cmds)
		output := buf.String()

		g.Expect(output).To(ContainSubstring("supersedes"), "primary should show supersedes annotation")
		g.Expect(output).To(ContainSubstring("superseded by"), "superseded entry should show annotation")
		g.Expect(output).To(ContainSubstring("deploy"), "non-duplicate should appear normally")
	})

	t.Run("SingleModuleHelpUnaffected", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		cmds := []runner.ExportCmdEntry{
			{Name: "build", Description: "build stuff", Source: "/project"},
			{Name: "test", Description: "test stuff", Source: "/project"},
		}

		var buf bytes.Buffer
		runner.ExportWriteCommandList(&buf, cmds)
		output := buf.String()

		g.Expect(output).NotTo(ContainSubstring("supersede"))
		g.Expect(output).To(ContainSubstring("build"))
		g.Expect(output).To(ContainSubstring("test"))
	})
}
