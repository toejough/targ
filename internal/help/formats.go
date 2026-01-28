// Package help format registry.
// This file provides centralized format definitions that can be subsetted per command.

package help

// FormatRegistry holds all known value format descriptions.
// Commands can reference these by name to include only relevant formats.
type FormatRegistry struct {
	formats map[string]Format
}

// NewFormatRegistry creates a new format registry with standard formats.
func NewFormatRegistry() *FormatRegistry {
	return &FormatRegistry{
		formats: map[string]Format{
			"duration": {
				Name: "duration",
				Desc: "<int><unit> where unit is s (seconds), m (minutes), h (hours)",
			},
		},
	}
}

// Get returns the format definition for the given name.
func (r *FormatRegistry) Get(name string) (Format, bool) {
	f, ok := r.formats[name]
	return f, ok
}
