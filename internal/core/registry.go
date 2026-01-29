package core

import (
	"fmt"
	"strings"
)

// Conflict represents a single target name conflict.
type Conflict struct {
	Name    string   // Target name with conflict
	Sources []string // Package paths that registered this name
}

// ConflictError represents duplicate target names from different sources.
type ConflictError struct {
	Conflicts []Conflict // All conflicts found
}

func (e *ConflictError) Error() string {
	var builder strings.Builder

	for i, conflict := range e.Conflicts {
		if i > 0 {
			builder.WriteString("\n")
		}

		builder.WriteString(fmt.Sprintf("targ: conflict: %q registered by both:", conflict.Name))

		for _, src := range conflict.Sources {
			builder.WriteString("\n  - ")
			builder.WriteString(src)
		}

		builder.WriteString("\nUse targ.DeregisterFrom() to resolve.")
	}

	return builder.String()
}

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

// resolveRegistry processes the global registry by applying deregistrations
// and detecting conflicts. Returns the filtered registry, deregistered package
// paths, or an error. Clears the deregistration queue after processing.
func (s *RegistryState) resolveRegistry() ([]any, []string, error) {
	s.registryResolved = true

	deregisteredPkgs := make([]string, 0, len(s.deregistrations))
	for _, dereg := range s.deregistrations {
		deregisteredPkgs = append(deregisteredPkgs, dereg.PackagePath)
	}

	defer func() {
		s.deregistrations = nil
	}()

	filtered, err := applyDeregistrations(s.registry, s.deregistrations)
	if err != nil {
		return nil, nil, err
	}

	// Then check for conflicts
	err = detectConflicts(filtered)
	if err != nil {
		return nil, nil, err
	}

	clearLocalTargetSources(filtered, s.mainModuleProvider)

	return filtered, deregisteredPkgs, nil
}

// applyDeregistrations filters out targets from specified packages.
// Only removes items at indices less than the RegistryLen specified in each deregistration.
// This allows re-registering targets after deregistering their package.
// Returns filtered registry and error if a deregistered package had no matches.
func applyDeregistrations(items []any, deregistrations []Deregistration) ([]any, error) {
	// Early return for empty deregistrations
	if len(deregistrations) == 0 {
		return items, nil
	}

	// Track which packages had matches
	matchCounts := initMatchCounts(deregistrations)

	// Filter items based on deregistrations
	result := filterItems(items, deregistrations, matchCounts)

	// Verify all packages had matches
	err := verifyMatchCounts(deregistrations, matchCounts)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// clearLocalTargetSources clears sourcePkg for targets from the main module.
// Uses mainModuleProvider to determine the main module path.
func clearLocalTargetSources(items []any, mainModuleProvider func() (string, bool)) {
	// Get main module path
	var mainModule string
	if mainModuleProvider != nil {
		mainModule, _ = mainModuleProvider()
	}

	// If we can't determine main module, leave all sources as-is
	if mainModule == "" {
		return
	}

	// Clear sourcePkg for any target or group from main module
	for _, item := range items {
		if target, ok := item.(*Target); ok {
			if isFromModule(target.sourcePkg, mainModule) {
				target.sourcePkg = ""
			}
		}

		if group, ok := item.(*TargetGroup); ok {
			if isFromModule(group.sourcePkg, mainModule) {
				group.sourcePkg = ""
			}
		}
	}
}

// detectConflicts checks the registry for name conflicts across packages.
// Returns nil if no conflicts, or *ConflictError with all conflicts found.
func detectConflicts(items []any) error {
	// Track name -> sources mapping
	nameSources := make(map[string]map[string]bool)

	for _, item := range items {
		var name, source string

		// Check Target items
		if target, ok := item.(*Target); ok {
			name = target.GetName()
			source = target.sourcePkg
		} else if group, ok := item.(*TargetGroup); ok {
			// Check TargetGroup items
			name = group.GetName()
			source = group.sourcePkg
		} else {
			// Skip non-Target, non-TargetGroup items
			continue
		}

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

// filterItems processes each item and decides whether to keep or remove it.
func filterItems(items []any, deregistrations []Deregistration, matchCounts map[string]int) []any {
	result := make([]any, 0, len(items))

	for idx, item := range items {
		sourcePkg := getSourcePkg(item)
		if sourcePkg == "" {
			// Non-Target, non-TargetGroup items pass through
			result = append(result, item)
			continue
		}

		// Check if item should be removed by any deregistration
		if shouldRemove(sourcePkg, idx, deregistrations, matchCounts) {
			continue
		}

		result = append(result, item)
	}

	return result
}

// getSourcePkg extracts the source package from a Target or TargetGroup.
// Returns empty string for other types.
func getSourcePkg(item any) string {
	switch v := item.(type) {
	case *Target:
		return v.sourcePkg
	case *TargetGroup:
		return v.sourcePkg
	default:
		return ""
	}
}

// initMatchCounts creates a map to track which packages had matches.
func initMatchCounts(deregistrations []Deregistration) map[string]int {
	matchCounts := make(map[string]int)
	for _, dereg := range deregistrations {
		matchCounts[dereg.PackagePath] = 0
	}

	return matchCounts
}

// isFromModule checks if a package path belongs to the given module.
// A package belongs to a module if it equals the module path or is a sub-package
// (has the module path followed by "/").
func isFromModule(pkgPath, modulePath string) bool {
	return pkgPath == modulePath || strings.HasPrefix(pkgPath, modulePath+"/")
}

// shouldRemove checks if an item should be removed based on deregistrations.
func shouldRemove(
	sourcePkg string,
	idx int,
	deregistrations []Deregistration,
	matchCounts map[string]int,
) bool {
	for _, dereg := range deregistrations {
		// Only remove if:
		// 1. Package matches AND
		// 2. Item was registered before the deregistration (idx < RegistryLen)
		if sourcePkg == dereg.PackagePath && idx < dereg.RegistryLen {
			matchCounts[dereg.PackagePath]++
			return true
		}
	}

	return false
}

// verifyMatchCounts checks that all deregistered packages had at least one match.
func verifyMatchCounts(deregistrations []Deregistration, matchCounts map[string]int) error {
	for _, dereg := range deregistrations {
		if matchCounts[dereg.PackagePath] == 0 {
			return &DeregistrationError{PackagePath: dereg.PackagePath}
		}
	}

	return nil
}
