//go:build targ_example

// Package local demonstrates consuming portable target packages.
// Shows three tiers: blank import, selective registration, and conflict resolution.
package local

import (
	"github.com/toejough/targ"

	// Tier 1 (simplest): Blank import - registers all remote targets
	// Uncomment to use:
	// _ "github.com/toejough/targ/examples/portable/remote"

	// Tier 2 (selective): Named import with selective registration
	// This is the ACTIVE tier in this example
	remote "github.com/toejough/targ/examples/portable/remote"
	// Tier 3 (conflict resolution): Multiple remotes with conflicts
	// Uncomment to demonstrate:
	// remote2 "github.com/other/targets"  // hypothetical second remote
)

// Tier 2 example (ACTIVE):
// - Import the remote package with a name
// - Deregister everything from that package
// - Selectively re-register only what you want
//
//nolint:gochecknoinits // init required for targ.Register - targets auto-register on import
func init() {
	// Deregister all targets from the remote package
	_ = targ.DeregisterFrom("github.com/toejough/targ/examples/portable/remote")

	// Re-register only the targets we want
	targ.Register(
		remote.Lint, // Use remote's lint
		remote.Test, // Use remote's test
		// Note: NOT registering remote.Build - we don't want it
	)

	// Add our own local targets
	targ.Register(LocalDeploy)
}

// Exported variables.
var (
	LocalDeploy = targ.Targ(deploy).
		Name("deploy").
		Description("Deploy after linting and testing").
		Deps(remote.Lint, remote.Test)
)

func deploy() {
	println("Deploying...")
}
