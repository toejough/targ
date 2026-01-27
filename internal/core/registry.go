package core

import (
	"fmt"
	"strings"
)

// DeregistrationError represents an error when deregistering a package with no targets.
type DeregistrationError struct {
	PackagePath string
}

func (e *DeregistrationError) Error() string {
	return fmt.Sprintf(
		"targ: DeregisterFrom(%q): no targets registered from this package",
		e.PackagePath,
	)
}

// ConflictError represents duplicate target names from different sources.
type ConflictError struct {
	Conflicts []Conflict // All conflicts found
}

// Conflict represents a single target name conflict.
type Conflict struct {
	Name    string   // Target name with conflict
	Sources []string // Package paths that registered this name
}

func (e *ConflictError) Error() string {
	var sb strings.Builder

	for i, conflict := range e.Conflicts {
		if i > 0 {
			sb.WriteString("\n")
		}

		sb.WriteString(fmt.Sprintf("targ: conflict: %q registered by both:", conflict.Name))

		for _, src := range conflict.Sources {
			sb.WriteString("\n  - ")
			sb.WriteString(src)
		}

		sb.WriteString("\nUse targ.DeregisterFrom() to resolve.")
	}

	return sb.String()
}

// applyDeregistrations filters out targets from specified packages.
// Returns filtered registry and error if a deregistered package had no matches.
func applyDeregistrations(items []any, packagePaths []string) ([]any, error) {
	// Early return for empty deregistrations
	if len(packagePaths) == 0 {
		return items, nil
	}

	// Track which packages had matches
	matchCounts := make(map[string]int)
	for _, pkg := range packagePaths {
		matchCounts[pkg] = 0
	}

	// Filter out targets from deregistered packages
	result := make([]any, 0, len(items))
	for _, item := range items {
		// Check if item is a Target
		target, ok := item.(*Target)
		if !ok {
			// Non-Target items pass through
			result = append(result, item)
			continue
		}

		// Check if target's package is in deregistration list
		shouldRemove := false

		for _, pkg := range packagePaths {
			if target.sourcePkg == pkg {
				shouldRemove = true
				matchCounts[pkg]++

				break
			}
		}

		if !shouldRemove {
			result = append(result, item)
		}
	}

	// Check for packages with no matches
	for _, pkg := range packagePaths {
		if matchCounts[pkg] == 0 {
			return nil, &DeregistrationError{PackagePath: pkg}
		}
	}

	return result, nil
}

// detectConflicts checks the registry for name conflicts across packages.
// Returns nil if no conflicts, or *ConflictError with all conflicts found.
func detectConflicts(items []any) error {
	// Track name -> sources mapping
	nameSources := make(map[string]map[string]bool)

	for _, item := range items {
		// Only check Target items
		target, ok := item.(*Target)
		if !ok {
			continue
		}

		name := target.GetName()
		source := target.sourcePkg

		// Initialize map if needed
		if nameSources[name] == nil {
			nameSources[name] = make(map[string]bool)
		}

		// Add source for this name
		nameSources[name][source] = true
	}

	// Collect all conflicts
	var conflicts []Conflict

	for name, sources := range nameSources {
		// Conflict only if multiple different sources
		if len(sources) > 1 {
			// Convert set to slice
			sourceList := make([]string, 0, len(sources))
			for src := range sources {
				sourceList = append(sourceList, src)
			}

			conflicts = append(conflicts, Conflict{
				Name:    name,
				Sources: sourceList,
			})
		}
	}

	// Return error if any conflicts found
	if len(conflicts) > 0 {
		return &ConflictError{Conflicts: conflicts}
	}

	return nil
}
