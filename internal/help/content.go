// Package help content structures.
// This file defines the data types for help content elements.

package help

// Positional represents a positional argument in command usage.
type Positional struct {
	Name        string
	Placeholder string
	Required    bool
}

// Flag represents a command-line flag.
type Flag struct {
	Long        string
	Short       string
	Desc        string
	Placeholder string
	Required    bool
}

// Format represents a value format description (e.g., duration syntax).
type Format struct {
	Name string
	Desc string
}

// Subcommand represents a subcommand entry in help output.
type Subcommand struct {
	Name string
	Desc string
}

// Example represents a usage example with title and code.
type Example struct {
	Title string
	Code  string
}
