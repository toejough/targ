package core

import "fmt"

// DeregistrationError represents an error when deregistering a package with no targets.
type DeregistrationError struct {
	PackagePath string
}

func (e *DeregistrationError) Error() string {
	return fmt.Sprintf("targ: DeregisterFrom(%q): no targets registered from this package", e.PackagePath)
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
