package core

import (
	"fmt"
	"regexp"
)

// TargetGroup represents a named collection of targets that can be run together.
type TargetGroup struct {
	name    string
	members []any // *Target or *TargetGroup
}

// GetMembers returns the group's members.
func (g *TargetGroup) GetMembers() []any {
	return g.members
}

// GetName returns the group's CLI name.
func (g *TargetGroup) GetName() string {
	return g.name
}

// Group creates a named group containing the given members.
// Members can be *Target or *TargetGroup (for nested hierarchies).
//
//	var lint = core.Group("lint", lintFast, lintFull)
//	var dev = core.Group("dev", build, lint, test)
func Group(name string, members ...any) *TargetGroup {
	if name == "" {
		panic("targ.Group: name cannot be empty")
	}

	if !validGroupName.MatchString(name) {
		panic(fmt.Sprintf(
			"targ.Group: invalid name %q (must match %s)",
			name, validGroupName.String(),
		))
	}

	// Validate members are *Target or *TargetGroup
	for i, m := range members {
		switch m.(type) {
		case *Target, *TargetGroup:
			// ok
		default:
			panic(fmt.Sprintf(
				"targ.Group: member %d has invalid type %T (expected *Target or *TargetGroup)",
				i, m,
			))
		}
	}

	return &TargetGroup{
		name:    name,
		members: members,
	}
}

// unexported variables.
var (
	validGroupName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)
