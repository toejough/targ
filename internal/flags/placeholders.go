package flags

// Placeholder describes a value format for flag arguments.
type Placeholder struct {
	Name   string // Display name in help, e.g., "<duration>"
	Format string // Format description, e.g., "30s, 5m, 1h"
}

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

func placeholderCmd() Placeholder {
	return Placeholder{Name: "<cmd>"}
}

func placeholderDir() Placeholder {
	return Placeholder{Name: "<dir>"}
}

func placeholderDuration() Placeholder {
	return Placeholder{Name: "<duration>", Format: "time value like 30s, 5m, 1h"}
}

func placeholderDurationMult() Placeholder {
	return Placeholder{
		Name:   "<duration,mult>",
		Format: "duration and multiplier like 1s,2.0",
	}
}

func placeholderGlob() Placeholder {
	return Placeholder{
		Name:   "<glob>",
		Format: "glob pattern like **/*.go, src/**",
	}
}

func placeholderMode() Placeholder {
	return Placeholder{Name: "{serial|parallel}"}
}

func placeholderN() Placeholder {
	return Placeholder{Name: "<n>"}
}

func placeholderShell() Placeholder {
	return Placeholder{Name: "{bash|zsh|fish}"}
}
