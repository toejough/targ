package help

import (
	"fmt"
	"io"
	"strings"
)

// StripANSI removes ANSI escape codes from a string for length calculation.
func StripANSI(s string) string {
	var result strings.Builder

	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}

		if inEscape {
			if r == 'm' {
				inEscape = false
			}

			continue
		}

		result.WriteRune(r)
	}

	return result.String()
}

// WriteIndentedLine writes a line with 2-space indentation.

// WriteExample writes an example with optional title.
// If title is non-empty: "  title:\n    code"
// If title is empty: "  code"
func WriteExample(w io.Writer, title, code string) {
	if title != "" {
		_, _ = fmt.Fprintf(w, "  %s:\n    %s\n", title, code)
	} else {
		_, _ = fmt.Fprintf(w, "  %s\n", code)
	}
}

// WriteSubheader writes a subsection header (bold, indented) followed by newline.

// WriteFlagLine writes a flag entry with optional short form, placeholder, and description.
// Format: "  --long, -s <placeholder>  description"
func WriteFlagLine(w io.Writer, long, short, placeholder, desc string) {
	WriteFlagLineIndent(w, long, short, placeholder, desc, "  ")
}

// WriteFlagLineIndent writes a flag entry with custom indentation.
func WriteFlagLineIndent(w io.Writer, long, short, placeholder, desc, indent string) {
	var nameParts []string
	if long != "" {
		nameParts = append(nameParts, styles.Flag.Render("--"+long))
	}

	if short != "" {
		nameParts = append(nameParts, styles.Flag.Render("-"+short))
	}

	line := indent + strings.Join(nameParts, ", ")
	if placeholder != "" {
		line += " " + styles.Placeholder.Render(placeholder)
	}

	// Pad to align descriptions
	const minWidth = 30

	visibleLen := len(StripANSI(line))
	if visibleLen < minWidth {
		line += strings.Repeat(" ", minWidth-visibleLen)
	} else {
		line += "  "
	}

	line += desc
	_, _ = fmt.Fprintln(w, line)
}

// WriteFormatLine writes a format entry with name and description.
// Format: "  name  description"
func WriteFormatLine(w io.Writer, name, desc string) {
	_, _ = fmt.Fprintf(w, "  %s  %s\n", styles.Placeholder.Render(name), desc)
}

// WriteHeader writes a section header (bold) followed by newline.
func WriteHeader(w io.Writer, text string) {
	_, _ = fmt.Fprintln(w, styles.Header.Render(text))
}

// WriteValueLine writes a value entry with name and description.
// Format: "  name: description"
func WriteValueLine(w io.Writer, name, desc string) {
	_, _ = fmt.Fprintf(w, "  %s: %s\n", styles.Placeholder.Render(name), desc)
}

// unexported variables.
var (
	//nolint:gochecknoglobals // Shared default styles for helper formatting.
	styles = DefaultStyles()
)
