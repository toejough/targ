package help

import "strings"

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

// No additional helpers in this file.
