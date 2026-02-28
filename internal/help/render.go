package help

import (
	"fmt"
	"strings"
)

// Render produces the final help string with all sections in canonical order.
func (cb *ContentBuilder) Render() string {
	styles := DefaultStyles()
	sections := cb.renderHeaderSections()
	sections = append(sections, cb.renderUsage(styles))
	sections = append(sections, cb.renderDynamicSections(styles)...)

	return strings.Join(sections, "\n\n") + "\n"
}

// renderBinaryModeFlags writes a flat "Flags:" section for binary mode.
func (cb *ContentBuilder) renderBinaryModeFlags(sb *strings.Builder, styles Styles) {
	sb.WriteString(styles.Header.Render("Flags:"))

	for _, f := range cb.globalFlags {
		sb.WriteString("\n")
		sb.WriteString(cb.renderFlag(f, styles))
	}

	for _, f := range cb.rootOnlyFlags {
		sb.WriteString("\n")
		sb.WriteString(cb.renderFlag(f, styles))
	}
}

// renderCommandFlags renders target-specific flags.
func (cb *ContentBuilder) renderCommandFlags(styles Styles) string {
	if len(cb.commandFlags) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(styles.Header.Render("Flags:"))

	for _, f := range cb.commandFlags {
		sb.WriteString("\n")
		sb.WriteString(cb.renderFlag(f, styles))
	}

	return sb.String()
}

func (cb *ContentBuilder) renderCommandGroups(styles Styles) string {
	if len(cb.commandGroups) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(styles.Header.Render("Commands:"))

	for i, group := range cb.commandGroups {
		if i > 0 {
			sb.WriteString("\n")
		}

		sb.WriteString("\n\n  Source: " + group.Source)

		// Calculate max name width for alignment
		maxWidth := 0
		for _, cmd := range group.Commands {
			if len(cmd.Name) > maxWidth {
				maxWidth = len(cmd.Name)
			}
		}

		for _, cmd := range group.Commands {
			sb.WriteString("\n  ")

			if cmd.Desc != "" {
				sb.WriteString(fmt.Sprintf("%-*s  %s", maxWidth, cmd.Name, cmd.Desc))
			} else {
				sb.WriteString(cmd.Name)
			}
		}
	}

	return sb.String()
}

func (cb *ContentBuilder) renderDynamicSections(styles Styles) []string {
	renderers := []func(Styles) string{
		cb.renderTargFlags,
		cb.renderValues,
		cb.renderFormats,
		cb.renderPositionals,
		cb.renderCommandFlags,
		cb.renderSubcommands,
		cb.renderCommandGroups,
		cb.renderExecutionInfo,
		cb.renderExamples,
	}

	var sections []string

	for _, render := range renderers {
		if s := render(styles); s != "" {
			sections = append(sections, s)
		}
	}

	if cb.moreInfoText != "" {
		sections = append(sections, cb.renderMoreInfo(styles))
	}

	return sections
}

func (cb *ContentBuilder) renderExamples(styles Styles) string {
	// If examplesSet but empty, user explicitly disabled examples
	if cb.examplesSet && len(cb.examples) == 0 {
		return ""
	}
	// If not set and empty, omit section
	if len(cb.examples) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(styles.Header.Render("Examples:"))

	for _, e := range cb.examples {
		sb.WriteString("\n  ")

		if e.Title != "" {
			sb.WriteString(e.Title + ":\n    ")
		}

		sb.WriteString(e.Code)
	}

	return sb.String()
}

func (cb *ContentBuilder) renderExecutionInfo(styles Styles) string {
	if cb.executionInfo == nil {
		return ""
	}

	info := cb.executionInfo

	var lines []string

	if info.Deps != "" {
		lines = append(lines, "Deps: "+info.Deps)
	}

	if info.CachePatterns != "" {
		lines = append(lines, "Cache: "+info.CachePatterns)
	}

	if info.WatchPatterns != "" {
		lines = append(lines, "Watch: "+info.WatchPatterns)
	}

	if info.Timeout != "" {
		lines = append(lines, "Timeout: "+info.Timeout)
	}

	if info.Times != "" {
		lines = append(lines, "Times: "+info.Times)
	}

	if info.Retry != "" {
		lines = append(lines, "Retry: "+info.Retry)
	}

	if len(lines) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(styles.Header.Render("Execution:"))

	for _, line := range lines {
		sb.WriteString("\n  ")
		sb.WriteString(line)
	}

	return sb.String()
}

func (cb *ContentBuilder) renderFlag(f Flag, styles Styles) string {
	return cb.renderFlagWithIndent(f, styles, "  ")
}

func (cb *ContentBuilder) renderFlagWithIndent(f Flag, styles Styles, indent string) string {
	var parts []string
	if f.Long != "" {
		parts = append(parts, styles.Flag.Render(f.Long))
	}

	if f.Short != "" {
		parts = append(parts, styles.Flag.Render(f.Short))
	}

	line := indent + strings.Join(parts, ", ")
	if f.Placeholder != "" {
		line += " " + styles.Placeholder.Render(f.Placeholder)
	}

	// Pad to align descriptions
	const minWidth = 30

	visibleLen := len(StripANSI(line))
	if visibleLen < minWidth {
		line += strings.Repeat(" ", minWidth-visibleLen)
	} else {
		line += "  "
	}

	if f.Desc != "" {
		line += f.Desc
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

// Render produces the final help string with all sections in canonical order.
// Section order:
//   - Description
//   - Source (target help only)
//   - Command (shell targets only)
//   - Usage
//   - Targ flags (grouped: Global, Root-only)
//   - Values (root help only)
//   - Formats
//   - Positionals (flag-command help)
//   - Flags (target-specific flags)
//   - Subcommands (target help)
//   - Commands (root help)
//   - Execution (target help)
//   - Examples
//   - More info
func (cb *ContentBuilder) renderHeaderSections() []string {
	var sections []string

	if cb.description != "" {
		sections = append(sections, cb.description)
	}

	if cb.sourceFile != "" {
		sections = append(sections, "Source: "+cb.sourceFile)
	}

	if cb.shellCommand != "" {
		sections = append(sections, "Command: "+cb.shellCommand)
	}

	return sections
}

func (cb *ContentBuilder) renderMoreInfo(styles Styles) string {
	var sb strings.Builder

	sb.WriteString(styles.Header.Render("More info:"))
	sb.WriteString("\n  ")
	sb.WriteString(cb.moreInfoText)

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

// renderTargFlags renders targ's built-in flags grouped by category.
func (cb *ContentBuilder) renderTargFlags(styles Styles) string {
	hasGlobal := len(cb.globalFlags) > 0
	hasRootOnly := len(cb.rootOnlyFlags) > 0

	if !hasGlobal && !hasRootOnly {
		return ""
	}

	var sb strings.Builder

	if cb.binaryMode {
		cb.renderBinaryModeFlags(&sb, styles)
	} else {
		cb.renderTargModeFlags(&sb, styles, hasGlobal, hasRootOnly)
	}

	return sb.String()
}

// renderTargModeFlags writes "Global flags:" with subsections for targ CLI mode.
func (cb *ContentBuilder) renderTargModeFlags(
	sb *strings.Builder,
	styles Styles,
	hasGlobal, hasRootOnly bool,
) {
	sb.WriteString(styles.Header.Render("Global flags:"))

	if hasGlobal {
		sb.WriteString("\n  ")
		sb.WriteString(styles.Subsection.Render("Global:"))

		for _, f := range cb.globalFlags {
			sb.WriteString("\n")
			sb.WriteString(cb.renderFlagWithIndent(f, styles, "    "))
		}
	}

	if hasRootOnly && cb.isRoot {
		sb.WriteString("\n  ")
		sb.WriteString(styles.Subsection.Render("Root only:"))

		for _, f := range cb.rootOnlyFlags {
			sb.WriteString("\n")
			sb.WriteString(cb.renderFlagWithIndent(f, styles, "    "))
		}
	}
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

func (cb *ContentBuilder) renderValues(styles Styles) string {
	if len(cb.values) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(styles.Header.Render("Values:"))

	for _, v := range cb.values {
		sb.WriteString("\n  ")
		sb.WriteString(styles.Placeholder.Render(v.Name))
		sb.WriteString(": ")
		sb.WriteString(v.Desc)
	}

	return sb.String()
}
