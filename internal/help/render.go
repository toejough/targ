// Package help rendering functions.
// This file handles the actual rendering of help content with proper styling.

package help

import (
	"fmt"
	"strings"
)

// Render produces the final help string with all sections in canonical order.
// Panics if AddExamples was not called (examples are required).
func (cb *ContentBuilder) Render() string {
	if len(cb.examples) == 0 {
		panic("help.Render: AddExamples must be called before Render")
	}

	styles := DefaultStyles()
	var sections []string

	// Description (always present, no header)
	if cb.description != "" {
		sections = append(sections, cb.description)
	}

	// Usage
	sections = append(sections, cb.renderUsage(styles))

	// Positionals (omitted if empty)
	if pos := cb.renderPositionals(styles); pos != "" {
		sections = append(sections, pos)
	}

	// Flags (omitted if empty)
	if flags := cb.renderFlags(styles); flags != "" {
		sections = append(sections, flags)
	}

	// Formats (omitted if empty)
	if formats := cb.renderFormats(styles); formats != "" {
		sections = append(sections, formats)
	}

	// Subcommands (omitted if empty)
	if subs := cb.renderSubcommands(styles); subs != "" {
		sections = append(sections, subs)
	}

	// Examples (always present)
	sections = append(sections, cb.renderExamples(styles))

	return strings.Join(sections, "\n\n") + "\n"
}

func (cb *ContentBuilder) renderUsage(styles Styles) string {
	var sb strings.Builder
	sb.WriteString(styles.Header.Render("Usage:"))
	sb.WriteString("\n")
	usage := cb.usage
	if usage == "" {
		usage = cb.commandName + " [options]"
	}
	sb.WriteString("  ")
	sb.WriteString(usage)
	return sb.String()
}

func (cb *ContentBuilder) renderPositionals(styles Styles) string {
	if len(cb.positionals) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(styles.Header.Render("Positionals:"))

	for _, p := range cb.positionals {
		sb.WriteString("\n  ")
		if p.Placeholder != "" {
			sb.WriteString(styles.Placeholder.Render(p.Placeholder))
		} else {
			sb.WriteString(styles.Placeholder.Render("<" + p.Name + ">"))
		}
		if p.Required {
			sb.WriteString(" (required)")
		}
	}

	return sb.String()
}

func (cb *ContentBuilder) renderFlags(styles Styles) string {
	hasGlobal := len(cb.globalFlags) > 0
	hasCommand := len(cb.commandFlags) > 0

	if !hasGlobal && !hasCommand {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(styles.Header.Render("Flags:"))

	if hasGlobal {
		sb.WriteString("\n  ")
		sb.WriteString(styles.Subsection.Render("Global Flags:"))
		for _, f := range cb.globalFlags {
			sb.WriteString("\n")
			sb.WriteString(cb.renderFlag(f, styles))
		}
	}

	if hasCommand {
		sb.WriteString("\n  ")
		sb.WriteString(styles.Subsection.Render("Command Flags:"))
		for _, f := range cb.commandFlags {
			sb.WriteString("\n")
			sb.WriteString(cb.renderFlag(f, styles))
		}
	}

	return sb.String()
}

func (cb *ContentBuilder) renderFlag(f Flag, styles Styles) string {
	var parts []string
	if f.Short != "" {
		parts = append(parts, styles.Flag.Render(f.Short))
	}
	if f.Long != "" {
		parts = append(parts, styles.Flag.Render(f.Long))
	}

	line := "    " + strings.Join(parts, ", ")
	if f.Placeholder != "" {
		line += " " + styles.Placeholder.Render(f.Placeholder)
	}
	if f.Desc != "" {
		line += "  " + f.Desc
	}
	return line
}

func (cb *ContentBuilder) renderFormats(styles Styles) string {
	if len(cb.formats) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(styles.Header.Render("Formats:"))

	for _, f := range cb.formats {
		sb.WriteString("\n  ")
		sb.WriteString(styles.Placeholder.Render(f.Name))
		if f.Desc != "" {
			sb.WriteString("  ")
			sb.WriteString(f.Desc)
		}
	}

	return sb.String()
}

func (cb *ContentBuilder) renderSubcommands(styles Styles) string {
	if len(cb.subcommands) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(styles.Header.Render("Subcommands:"))

	for _, s := range cb.subcommands {
		sb.WriteString("\n  ")
		sb.WriteString(s.Name)
		if s.Desc != "" {
			sb.WriteString("  ")
			sb.WriteString(s.Desc)
		}
	}

	return sb.String()
}

func (cb *ContentBuilder) renderExamples(styles Styles) string {
	var sb strings.Builder
	sb.WriteString(styles.Header.Render("Examples:"))

	for _, e := range cb.examples {
		sb.WriteString("\n  ")
		if e.Title != "" {
			sb.WriteString(fmt.Sprintf("%s:\n    ", e.Title))
		}
		sb.WriteString(e.Code)
	}

	return sb.String()
}
