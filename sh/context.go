package sh

import (
	"context"

	internal "github.com/toejough/targ/internal/sh"
)

// OutputContext executes a command and returns combined output, with context support.
// When ctx is cancelled, the process and all its children are killed.
func OutputContext(ctx context.Context, name string, args ...string) (string, error) {
	return internal.OutputContext(ctx, name, args, internal.Stdin)
}

// RunContext executes a command with context support.
// When ctx is cancelled, the process and all its children are killed.
func RunContext(ctx context.Context, name string, args ...string) error {
	return internal.RunContextWithIO(ctx, name, args)
}

// RunContextV executes a command, prints it first, with context support.
// When ctx is cancelled, the process and all its children are killed.
func RunContextV(ctx context.Context, name string, args ...string) error {
	return internal.RunContextV(ctx, name, args)
}
