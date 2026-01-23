package core

import (
	"fmt"
	"regexp"
)

type Group struct {
	name    string
	members []any // *Target or *Group
}

// NewGroup creates a named group containing the given members.
// Members can be *Target or *Group (for nested hierarchies).
//
//	var lint = core.NewGroup("lint", lintFast, lintFull)
//	var dev = core.NewGroup("dev", build, lint, test)
func NewGroup(name string, members ...any) *Group {
	if name == "" {
		panic("targ.Group: name cannot be empty")
	}

	if !validGroupName.MatchString(name) {
		panic(fmt.Sprintf(
			"targ.Group: invalid name %q (must match %s)",
			name, validGroupName.String(),
		))
	}

	// Validate members are *Target or *Group
	for i, m := range members {
		switch m.(type) {
		case *Target, *Group:
			// ok
		default:
			panic(fmt.Sprintf(
				"targ.Group: member %d has invalid type %T (expected *Target or *Group)",
				i, m,
			))
		}
	}

	return &Group{
		name:    name,
		members: members,
	}
}

// GetMembers returns the group's members.
func (g *Group) GetMembers() []any {
	return g.members
}

// GetName returns the group's CLI name.
func (g *Group) GetName() string {
	return g.name
}

// unexported variables.
var (
	validGroupName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)
