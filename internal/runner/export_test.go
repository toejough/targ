package runner

import "io"

// Test-only exports for superseding tests.

// ExportCmdEntry is a test-visible wrapper for cmdEntry.
type ExportCmdEntry struct {
	Name         string
	Description  string
	Source       string
	SupersededBy string
	Supersedes   string
}

// ExportCommandInfo is a test-visible wrapper for commandInfo.
type ExportCommandInfo struct {
	Name        string
	Description string
}

// ExportModuleRegistry is a test-visible wrapper for moduleRegistry.
type ExportModuleRegistry struct {
	BinaryPath string
	ModuleRoot string
	ModulePath string
	Commands   []ExportCommandInfo
}

// ExportCollectSortedCommands wraps collectSortedCommands for testing.
func ExportCollectSortedCommands(registry []ExportModuleRegistry) []ExportCmdEntry {
	internal := make([]moduleRegistry, len(registry))
	for i, r := range registry {
		cmds := make([]commandInfo, len(r.Commands))
		for j, c := range r.Commands {
			cmds[j] = commandInfo(c)
		}

		internal[i] = moduleRegistry{
			BinaryPath: r.BinaryPath,
			ModuleRoot: r.ModuleRoot,
			ModulePath: r.ModulePath,
			Commands:   cmds,
		}
	}

	result := collectSortedCommands(internal)
	out := make([]ExportCmdEntry, len(result))

	for i, e := range result {
		out[i] = ExportCmdEntry{
			Name:         e.name,
			Description:  e.description,
			Source:       e.source,
			SupersededBy: e.supersededBy,
			Supersedes:   e.supersedes,
		}
	}

	return out
}

// ExportFindCommandBinary wraps findCommandBinary for testing.
func ExportFindCommandBinary(registry []ExportModuleRegistry, cmdName string) (string, bool) {
	internal := make([]moduleRegistry, len(registry))
	for i, r := range registry {
		cmds := make([]commandInfo, len(r.Commands))
		for j, c := range r.Commands {
			cmds[j] = commandInfo(c)
		}

		internal[i] = moduleRegistry{
			BinaryPath: r.BinaryPath,
			ModuleRoot: r.ModuleRoot,
			ModulePath: r.ModulePath,
			Commands:   cmds,
		}
	}

	return findCommandBinary(internal, cmdName)
}

// ExportWriteCommandList wraps writeCommandList for testing.
func ExportWriteCommandList(w io.Writer, entries []ExportCmdEntry) {
	internal := make([]cmdEntry, len(entries))
	for i, e := range entries {
		internal[i] = cmdEntry{
			name:         e.Name,
			description:  e.Description,
			source:       e.Source,
			supersededBy: e.SupersededBy,
			supersedes:   e.Supersedes,
		}
	}

	writeCommandList(w, internal)
}
