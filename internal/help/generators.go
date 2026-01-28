package help

import (
	"fmt"
	"io"
)

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
	b := New(opts.BinaryName).
		WithDescription(opts.Description).
		WithUsage(opts.BinaryName + " [targ flags...] [<command>...]").
		SetRoot(true).
		AddTargFlagsFiltered(opts.Filter)

	// Commands grouped by source
	b.AddCommandGroups(opts.CommandGroups...)

	// Examples
	if len(opts.Examples) > 0 {
		b.AddExamples(opts.Examples...)
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
		b.WithUsage(fmt.Sprintf("%s [targ flags...] %s [flags...]", opts.BinaryName, opts.Name))
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

	// Examples (only if explicitly provided)
	if len(opts.Examples) > 0 {
		b.AddExamples(opts.Examples...)
	} else {
		// Mark as explicitly empty to omit Examples section
		b.AddExamples()
	}

	// More info
	if opts.MoreInfoText != "" {
		b.WithMoreInfo(opts.MoreInfoText)
	}

	output := b.Render()
	_, _ = fmt.Fprint(w, output)
}
