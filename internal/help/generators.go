package help

import (
	"fmt"
	"io"
	"strings"
)

// GenerateRootExamples creates examples from command metadata.
func GenerateRootExamples(binaryName string, groups []CommandGroup, binaryMode bool) []Example {
	// Collect all command names
	var cmdNames []string
	for _, g := range groups {
		for _, c := range g.Commands {
			cmdNames = append(cmdNames, c.Name)
		}
	}

	if len(cmdNames) == 0 {
		return nil
	}

	var examples []Example

	// Basic: run a command
	examples = append(examples, Example{
		Title: "Run a command",
		Code:  binaryName + " " + cmdNames[0],
	})

	// Chain: run multiple commands (targ mode only, 2+ commands needed)
	if !binaryMode && len(cmdNames) >= 2 {
		examples = append(examples, Example{
			Title: "Chain commands",
			Code:  binaryName + " " + cmdNames[0] + " " + cmdNames[1],
		})
	}

	return examples
}

// RootHelpOpts contains options for generating root-level help.
type RootHelpOpts struct {
	BinaryName           string
	Description          string
	CommandGroups        []CommandGroup
	DeregisteredPackages []string
	Examples             []Example
	MoreInfoText         string
	Filter               TargFlagFilter
}

// TargetHelpOpts contains options for generating target-level help.
type TargetHelpOpts struct {
	BinaryName    string
	Name          string
	Description   string
	SourceFile    string
	ShellCommand  string
	Usage         string
	Flags         []Flag
	Subcommands   []Subcommand
	ExecutionInfo *ExecutionInfo
	Examples      []Example
	MoreInfoText  string
	Filter        TargFlagFilter
}

// WriteRootHelp writes the root-level help (targ --help) to w.
func WriteRootHelp(w io.Writer, opts RootHelpOpts) {
	// Use "[flags...]" in binary mode, "[targ flags...]" otherwise
	usageFlags := "[targ flags...]"
	if opts.Filter.BinaryMode {
		usageFlags = "[flags...]"
	}

	b := New(opts.BinaryName).
		WithDescription(opts.Description).
		WithUsage(opts.BinaryName + " " + usageFlags + " [<command>...]").
		SetRoot(true).
		AddTargFlagsFiltered(opts.Filter)

	// Commands grouped by source
	b.AddCommandGroups(opts.CommandGroups...)

	// Examples (auto-generate if not provided by user)
	examples := opts.Examples
	if len(examples) == 0 {
		examples = GenerateRootExamples(opts.BinaryName, opts.CommandGroups, opts.Filter.BinaryMode)
	}
	if len(examples) > 0 {
		b.AddExamples(examples...)
	}

	// More info
	if opts.MoreInfoText != "" {
		b.WithMoreInfo(opts.MoreInfoText)
	}

	output := b.Render()
	_, _ = fmt.Fprint(w, output)

	// Deregistered packages (separate from Builder since it's a special case)
	if len(opts.DeregisteredPackages) > 0 {
		_, _ = fmt.Fprintln(
			w,
			"\nDeregistered packages (targets hidden — edit init() in your targ file to re-register):",
		)

		for _, pkg := range opts.DeregisteredPackages {
			_, _ = fmt.Fprintf(w, "  %s\n", pkg)
		}
	}
}

// WriteTargetHelp writes target-level help (targ <target> --help) to w.
func WriteTargetHelp(w io.Writer, opts TargetHelpOpts) {
	b := New(opts.Name).
		WithDescription(opts.Description)

	// Source file
	if opts.SourceFile != "" {
		b.WithSourceFile(opts.SourceFile)
	}

	// Shell command
	if opts.ShellCommand != "" {
		b.WithShellCommand(opts.ShellCommand)
	}

	// Usage
	if opts.Usage != "" {
		b.WithUsage(opts.Usage)
	} else {
		// Use "[flags...]" in binary mode, "[targ flags...]" otherwise
		usageFlags := "[targ flags...]"
		if opts.Filter.BinaryMode {
			usageFlags = "[flags...]"
		}
		b.WithUsage(fmt.Sprintf("%s %s %s [flags...]", opts.BinaryName, usageFlags, opts.Name))
	}

	// Targ flags (non-root level)
	b.SetRoot(false).
		AddTargFlagsFiltered(opts.Filter)

	// Target-specific flags
	if len(opts.Flags) > 0 {
		b.AddCommandFlags(opts.Flags...)
	}

	// Subcommands
	if len(opts.Subcommands) > 0 {
		b.AddSubcommands(opts.Subcommands...)
	}

	// Execution info
	if opts.ExecutionInfo != nil {
		b.WithExecutionInfo(*opts.ExecutionInfo)
	}

	// Examples (auto-generate if not provided by user)
	if len(opts.Examples) > 0 {
		b.AddExamples(opts.Examples...)
	} else {
		// Always generate at least basic usage example
		generated := GenerateTargetExamples(opts.BinaryName, opts.Name, opts.Flags, opts.Filter.BinaryMode)
		b.AddExamples(generated...)
	}

	// More info
	if opts.MoreInfoText != "" {
		b.WithMoreInfo(opts.MoreInfoText)
	}

	output := b.Render()
	_, _ = fmt.Fprint(w, output)
}

// GenerateTargetExamples creates examples from target metadata.
func GenerateTargetExamples(binaryName, targetName string, cmdFlags []Flag, binaryMode bool) []Example {
	prefix := binaryName + " " + targetName
	if binaryMode {
		prefix = binaryName
		if targetName != "" {
			prefix += " " + targetName
		}
	}

	var examples []Example

	// Basic usage
	examples = append(examples, Example{
		Title: "Basic usage",
		Code:  prefix,
	})

	// With options (if there are non-required flags)
	var optionalFlags []Flag
	for _, f := range cmdFlags {
		if !f.Required {
			optionalFlags = append(optionalFlags, f)
		}
	}

	if len(optionalFlags) > 0 {
		var code strings.Builder
		code.WriteString(prefix)
		// Show up to 2 optional flags
		limit := min(2, len(optionalFlags))
		for _, f := range optionalFlags[:limit] {
			code.WriteString(" " + f.Long)
			if f.Placeholder != "" {
				code.WriteString(" " + exampleValueForPlaceholder(f.Placeholder))
			}
		}

		examples = append(examples, Example{
			Title: "With options",
			Code:  code.String(),
		})
	}

	return examples
}

func exampleValueForPlaceholder(placeholder string) string {
	switch strings.ToLower(placeholder) {
	case "n":
		return "10"
	case "duration":
		return "30s"
	case "d,m":
		return "1s,2.0"
	default:
		return strings.ToLower(placeholder)
	}
}
