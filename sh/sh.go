// Package sh provides utilities for running shell commands in build scripts.
package sh

import (
	"context"
	"os"

	internal "github.com/toejough/targ/internal/sh"
)

// EnableCleanup enables automatic cleanup of child processes on SIGINT/SIGTERM.
// Call this once at program startup to ensure Ctrl-C kills all spawned processes.
func EnableCleanup() {
	internal.EnableCleanup()
}

// ExeSuffix returns ".exe" on Windows, otherwise an empty string.
func ExeSuffix() string {
	return internal.ExeSuffix(nil)
}

// IsWindows reports whether the current OS is Windows.
func IsWindows() bool {
	return internal.IsWindowsOS()
}

// Output executes a command and returns combined output.
func Output(name string, args ...string) (string, error) {
	return internal.Output(nil, name, args...)
}

// OutputContext executes a command and returns combined output, with context support.
// When ctx is cancelled, the process and all its children are killed.
func OutputContext(ctx context.Context, name string, args ...string) (string, error) {
	return internal.OutputContext(ctx, name, args, os.Stdin)
}

// Run executes a command streaming stdout/stderr.
func Run(name string, args ...string) error {
	return internal.Run(nil, name, args...)
}

// RunContext executes a command with context support.
// When ctx is cancelled, the process and all its children are killed.
func RunContext(ctx context.Context, name string, args ...string) error {
	return internal.RunContextWithIO(ctx, nil, name, args)
}

// RunContextV executes a command, prints it first, with context support.
// When ctx is cancelled, the process and all its children are killed.
func RunContextV(ctx context.Context, name string, args ...string) error {
	return internal.RunContextV(ctx, nil, name, args)
}

// RunV executes a command and prints it first.
func RunV(name string, args ...string) error {
	return internal.RunV(nil, name, args...)
}

// WithExeSuffix appends the OS-specific executable suffix if missing.
func WithExeSuffix(name string) string {
	return internal.WithExeSuffix(nil, name)
}
