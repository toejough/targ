// Package help provides a type-safe builder for consistent CLI help output.
// It uses a type-state pattern to enforce correct section ordering at compile time.
package help

import "github.com/toejough/targ/internal/flags"

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

// WithDescription sets the command description and transitions to ContentBuilder.
// This is the first method that must be called after New().
func (b *Builder) WithDescription(desc string) *ContentBuilder {
	b.content.description = desc
	return b.content
}

// WithUsage sets a custom usage line for the help output.
func (cb *ContentBuilder) WithUsage(usage string) *ContentBuilder {
	cb.usage = usage
	return cb
}

// AddPositionals adds positional arguments to the help output.
// Multiple calls accumulate positionals.
func (cb *ContentBuilder) AddPositionals(pos ...Positional) *ContentBuilder {
	cb.positionals = append(cb.positionals, pos...)
	return cb
}

// AddGlobalFlags adds global flags by looking them up in the flag registry.
// Unknown flag names are silently ignored.
func (cb *ContentBuilder) AddGlobalFlags(flagNames ...string) *ContentBuilder {
	for _, name := range flagNames {
		def := flags.Find(name)
		if def == nil {
			continue // Unknown flag, skip
		}
		f := Flag{
			Long: "--" + def.Long,
			Desc: def.Desc,
		}
		if def.Short != "" {
			f.Short = "-" + def.Short
		}
		if def.TakesValue {
			f.Placeholder = "<value>"
		}
		cb.globalFlags = append(cb.globalFlags, f)
	}
	return cb
}

// AddCommandFlags adds command-specific flags to the help output.
// Multiple calls accumulate flags.
func (cb *ContentBuilder) AddCommandFlags(flags ...Flag) *ContentBuilder {
	cb.commandFlags = append(cb.commandFlags, flags...)
	return cb
}

// AddFormats adds value format descriptions to the help output.
// Multiple calls accumulate formats.
func (cb *ContentBuilder) AddFormats(formats ...Format) *ContentBuilder {
	cb.formats = append(cb.formats, formats...)
	return cb
}

// AddSubcommands adds subcommand entries to the help output.
// Multiple calls accumulate subcommands.
func (cb *ContentBuilder) AddSubcommands(subs ...Subcommand) *ContentBuilder {
	cb.subcommands = append(cb.subcommands, subs...)
	return cb
}

// AddExamples sets the examples for the help output.
// At least one example is required; panics if called with no arguments.
func (cb *ContentBuilder) AddExamples(examples ...Example) *ContentBuilder {
	if len(examples) == 0 {
		panic("help.AddExamples: at least one example is required")
	}
	cb.examples = append(cb.examples, examples...)
	return cb
}
