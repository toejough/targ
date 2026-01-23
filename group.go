package targ

import (
	"github.com/toejough/targ/internal/core"
)

type Group = core.Group

// NewGroup creates a named group containing the given members.
// Members can be *Target or *Group (for nested hierarchies).
//
//	var lint = targ.NewGroup("lint", lintFast, lintFull)
//	var dev = targ.NewGroup("dev", build, lint, test)
func NewGroup(name string, members ...any) *Group {
	return core.NewGroup(name, members...)
}
