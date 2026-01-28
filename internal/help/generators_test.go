package help_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/help"
)

func TestWriteRootHelpBasic(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteRootHelp(&buf, help.RootHelpOpts{
		BinaryName:  "targ",
		Description: "Test description",
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Test description"))
	g.Expect(output).To(ContainSubstring("Usage:"))
}

func TestWriteRootHelpWithDeregisteredPackages(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteRootHelp(&buf, help.RootHelpOpts{
		BinaryName:           "targ",
		Description:          "Test description",
		DeregisteredPackages: []string{"github.com/foo/bar", "github.com/baz/qux"},
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Deregistered packages"))
	g.Expect(output).To(ContainSubstring("github.com/foo/bar"))
	g.Expect(output).To(ContainSubstring("github.com/baz/qux"))
}

func TestWriteRootHelpWithExamples(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteRootHelp(&buf, help.RootHelpOpts{
		BinaryName:  "targ",
		Description: "Test description",
		Examples: []help.Example{
			{Title: "Basic", Code: "targ build"},
		},
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Examples:"))
	g.Expect(output).To(ContainSubstring("targ build"))
}

func TestWriteRootHelpWithMoreInfo(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteRootHelp(&buf, help.RootHelpOpts{
		BinaryName:   "targ",
		Description:  "Test description",
		MoreInfoText: "https://example.com",
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("More info:"))
	g.Expect(output).To(ContainSubstring("https://example.com"))
}

func TestWriteTargetHelpBasic(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteTargetHelp(&buf, help.TargetHelpOpts{
		BinaryName:  "targ",
		Name:        "build",
		Description: "Build the project",
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Build the project"))
	g.Expect(output).To(ContainSubstring("Usage:"))
}

func TestWriteTargetHelpWithCustomUsage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteTargetHelp(&buf, help.TargetHelpOpts{
		BinaryName:  "targ",
		Name:        "build",
		Description: "Build the project",
		Usage:       "targ build [--verbose] <target>",
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("targ build [--verbose] <target>"))
}

func TestWriteTargetHelpWithExamples(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteTargetHelp(&buf, help.TargetHelpOpts{
		BinaryName:  "targ",
		Name:        "build",
		Description: "Build the project",
		Examples: []help.Example{
			{Title: "Build all", Code: "targ build --all"},
		},
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Examples:"))
	g.Expect(output).To(ContainSubstring("Build all"))
}

func TestWriteTargetHelpWithExecutionInfo(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteTargetHelp(&buf, help.TargetHelpOpts{
		BinaryName:  "targ",
		Name:        "build",
		Description: "Build the project",
		ExecutionInfo: &help.ExecutionInfo{
			Deps: "generate, fmt (serial)",
		},
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Execution:"))
	g.Expect(output).To(ContainSubstring("generate, fmt"))
}

func TestWriteTargetHelpWithFlags(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteTargetHelp(&buf, help.TargetHelpOpts{
		BinaryName:  "targ",
		Name:        "build",
		Description: "Build the project",
		Flags: []help.Flag{
			{Long: "--verbose", Short: "-v", Desc: "Verbose output"},
		},
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Flags:"))
	g.Expect(output).To(ContainSubstring("--verbose"))
}

func TestWriteTargetHelpWithMoreInfo(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteTargetHelp(&buf, help.TargetHelpOpts{
		BinaryName:   "targ",
		Name:         "build",
		Description:  "Build the project",
		MoreInfoText: "https://docs.example.com",
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("More info:"))
	g.Expect(output).To(ContainSubstring("https://docs.example.com"))
}

func TestWriteTargetHelpWithShellCommand(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteTargetHelp(&buf, help.TargetHelpOpts{
		BinaryName:   "targ",
		Name:         "lint",
		Description:  "Lint the code",
		ShellCommand: "golangci-lint run ./...",
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Command:"))
	g.Expect(output).To(ContainSubstring("golangci-lint run"))
}

func TestWriteTargetHelpWithSourceFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteTargetHelp(&buf, help.TargetHelpOpts{
		BinaryName:  "targ",
		Name:        "build",
		Description: "Build the project",
		SourceFile:  "dev/targets.go",
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Source:"))
	g.Expect(output).To(ContainSubstring("dev/targets.go"))
}

func TestWriteTargetHelpWithSubcommands(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteTargetHelp(&buf, help.TargetHelpOpts{
		BinaryName:  "targ",
		Name:        "dev",
		Description: "Development commands",
		Subcommands: []help.Subcommand{
			{Name: "lint", Desc: "Run linters"},
		},
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Subcommands:"))
	g.Expect(output).To(ContainSubstring("lint"))
}
