// Package flags placeholder definitions.
// Placeholders are structured types that describe value formats for flags.
// Help text for placeholders is automatically generated from these definitions.

package flags

// Placeholder describes a value format for flag arguments.
type Placeholder struct {
	Name   string // Display name in help, e.g., "<duration>"
	Format string // Format description, e.g., "30s, 5m, 1h"
}

// Standard placeholders used by targ flags.
// Each placeholder that isn't self-evident should be defined here.
var (
	// PlaceholderDuration describes time duration values.
	PlaceholderDuration = Placeholder{
		Name:   "<duration>",
		Format: "time value like 30s, 5m, 1h",
	}

	// PlaceholderDurationMult describes duration with multiplier for backoff.
	PlaceholderDurationMult = Placeholder{
		Name:   "<duration,mult>",
		Format: "duration and multiplier like 1s,2.0",
	}

	// PlaceholderGlob describes glob patterns.
	PlaceholderGlob = Placeholder{
		Name:   "<glob>",
		Format: "glob pattern like **/*.go, src/**",
	}

	// Self-evident placeholders (no Format needed - obvious from context)
	PlaceholderN      = Placeholder{Name: "<n>"}
	PlaceholderDir    = Placeholder{Name: "<dir>"}
	PlaceholderCmd    = Placeholder{Name: "<cmd>"}
	PlaceholderShell  = Placeholder{Name: "{bash|zsh|fish}"}  // Enum, self-documenting
	PlaceholderMode   = Placeholder{Name: "{serial|parallel}"} // Enum, self-documenting
)

// NeedsExplanation returns true if this placeholder has a non-obvious format.
func (p Placeholder) NeedsExplanation() bool {
	return p.Format != ""
}

// PlaceholdersUsedByFlags returns unique placeholders that need explanation
// from the given flag definitions.
func PlaceholdersUsedByFlags(defs []Def) []Placeholder {
	seen := make(map[string]bool)
	var result []Placeholder

	for _, def := range defs {
		if def.Placeholder == nil || !def.Placeholder.NeedsExplanation() {
			continue
		}
		if seen[def.Placeholder.Name] {
			continue
		}
		seen[def.Placeholder.Name] = true
		result = append(result, *def.Placeholder)
	}

	return result
}
