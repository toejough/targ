package targ

import (
	"context"

	"github.com/toejough/targ/internal/core"
)

// Shell executes a shell command with variable substitution from struct fields.
// Variables are specified as $name in the command string and are replaced with
// the corresponding field value from the args struct.
//
// Example:
//
//	type DeployArgs struct {
//	    Namespace string
//	    File      string
//	}
//	err := targ.Shell(ctx, "kubectl apply -n $namespace -f $file", args)
//
// Field names are matched case-insensitively (e.g., $namespace matches Namespace).
// Unknown variables return an error.
func Shell(ctx context.Context, cmd string, args any) error {
	return core.Shell(ctx, cmd, args)
}
