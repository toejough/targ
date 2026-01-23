package sh

import (
	internal "github.com/toejough/targ/internal/sh"
)

// ExeSuffix returns ".exe" on Windows, otherwise an empty string.
func ExeSuffix() string {
	return internal.ExeSuffix()
}

// IsWindows reports whether the current OS is Windows.
func IsWindows() bool {
	return internal.IsWindows()
}

// Output executes a command and returns combined output.
func Output(name string, args ...string) (string, error) {
	return internal.Output(name, args...)
}

// Run executes a command streaming stdout/stderr.
func Run(name string, args ...string) error {
	return internal.Run(name, args...)
}

// RunV executes a command and prints it first.
func RunV(name string, args ...string) error {
	return internal.RunV(name, args...)
}

// WithExeSuffix appends the OS-specific executable suffix if missing.
func WithExeSuffix(name string) string {
	return internal.WithExeSuffix(name)
}
