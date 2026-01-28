// Package help provides a type-safe builder for consistent CLI help output.
// It uses a type-state pattern to enforce correct section ordering at compile time.
package help

// Builder is the entry point for constructing help output.
// It uses a type-state pattern where each phase returns a new type,
// ensuring sections are added in the correct order.
type Builder struct {
	content *ContentBuilder
}

// New creates a new help Builder for the given command name.
// It panics if commandName is empty.
func New(commandName string) *Builder {
	if commandName == "" {
		panic("help.New: commandName must not be empty")
	}
	return &Builder{
		content: &ContentBuilder{
			commandName: commandName,
		},
	}
}
