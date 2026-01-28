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

// AddCommandFlags adds command-specific flags to the help output.
// Multiple calls accumulate flags.
func (cb *ContentBuilder) AddCommandFlags(flags ...Flag) *ContentBuilder {
	cb.commandFlags = append(cb.commandFlags, flags...)
	return cb
}

// AddCommandGroups adds grouped command lists (for top-level help).
func (cb *ContentBuilder) AddCommandGroups(groups ...CommandGroup) *ContentBuilder {
	cb.commandGroups = append(cb.commandGroups, groups...)
	return cb
}

// AddExamples adds examples to the help output.
// If called with no arguments, explicitly disables examples.
func (cb *ContentBuilder) AddExamples(examples ...Example) *ContentBuilder {
	cb.examplesSet = true
	cb.examples = append(cb.examples, examples...)

	return cb
}

// AddFormats adds value format descriptions to the help output.
// Multiple calls accumulate formats.
func (cb *ContentBuilder) AddFormats(formats ...Format) *ContentBuilder {
	cb.formats = append(cb.formats, formats...)
	return cb
}

// AddGlobalFlags adds global flags (available at any command level).
func (cb *ContentBuilder) AddGlobalFlags(flgs ...Flag) *ContentBuilder {
	cb.globalFlags = append(cb.globalFlags, flgs...)
	return cb
}

// AddGlobalFlagsFromRegistry adds global flags by looking them up in the flag registry.
// Unknown flag names are silently ignored.
func (cb *ContentBuilder) AddGlobalFlagsFromRegistry(flagNames ...string) *ContentBuilder {
	for _, name := range flagNames {
		def := flags.Find(name)
		if def == nil {
			continue // Unknown flag, skip
		}

		f := FlagFromDef(def)
		cb.globalFlags = append(cb.globalFlags, f)
	}

	return cb
}

// AddPositionals adds positional arguments to the help output.
// Multiple calls accumulate positionals.
func (cb *ContentBuilder) AddPositionals(pos ...Positional) *ContentBuilder {
	cb.positionals = append(cb.positionals, pos...)
	return cb
}

// AddRootOnlyFlags adds root-only flags to the help output.
func (cb *ContentBuilder) AddRootOnlyFlags(flags ...Flag) *ContentBuilder {
	cb.rootOnlyFlags = append(cb.rootOnlyFlags, flags...)
	return cb
}

// AddSubcommands adds subcommand entries to the help output.
// Multiple calls accumulate subcommands.
func (cb *ContentBuilder) AddSubcommands(subs ...Subcommand) *ContentBuilder {
	cb.subcommands = append(cb.subcommands, subs...)
	return cb
}

// AddTargFlagsFiltered adds targ's built-in flags, filtered and grouped.
// Also automatically adds Formats for any placeholders that need explanation.
func (cb *ContentBuilder) AddTargFlagsFiltered(filter TargFlagFilter) *ContentBuilder {
	var includedDefs []flags.Def

	for _, def := range flags.VisibleFlags() {
		if shouldSkipTargFlag(def, filter) {
			continue
		}

		includedDefs = append(includedDefs, def)

		f := FlagFromDef(&def)
		if def.RootOnly {
			cb.rootOnlyFlags = append(cb.rootOnlyFlags, f)
		} else {
			cb.globalFlags = append(cb.globalFlags, f)
		}
	}

	// Automatically add Formats for placeholders that need explanation
	for _, ph := range flags.PlaceholdersUsedByFlags(includedDefs) {
		cb.formats = append(cb.formats, Format{
			Name: ph.Name,
			Desc: ph.Format,
		})
	}

	return cb
}

// AddValues adds value type descriptions to the help output.
func (cb *ContentBuilder) AddValues(values ...Value) *ContentBuilder {
	cb.values = append(cb.values, values...)
	return cb
}

// SetRoot marks this as root-level help (affects section visibility).
func (cb *ContentBuilder) SetRoot(isRoot bool) *ContentBuilder {
	cb.isRoot = isRoot
	return cb
}

// WithExecutionInfo sets the execution configuration display.
func (cb *ContentBuilder) WithExecutionInfo(info ExecutionInfo) *ContentBuilder {
	cb.executionInfo = &info
	return cb
}

// WithMoreInfo sets the more info text (URL or custom text).
func (cb *ContentBuilder) WithMoreInfo(text string) *ContentBuilder {
	cb.moreInfoText = text
	return cb
}

// WithShellCommand sets the shell command (for shell targets).
func (cb *ContentBuilder) WithShellCommand(cmd string) *ContentBuilder {
	cb.shellCommand = cmd
	return cb
}

// WithSourceFile sets the source file location (for target help).
func (cb *ContentBuilder) WithSourceFile(path string) *ContentBuilder {
	cb.sourceFile = path
	return cb
}

// WithUsage sets a custom usage line for the help output.
func (cb *ContentBuilder) WithUsage(usage string) *ContentBuilder {
	cb.usage = usage
	return cb
}

// TargFlagFilter controls which targ flags to include.
type TargFlagFilter struct {
	IsRoot            bool
	DisableCompletion bool
	DisableHelp       bool
	DisableTimeout    bool
}

// FlagFromDef creates a Flag from a flags.Def.
func FlagFromDef(def *flags.Def) Flag {
	f := Flag{
		Long: "--" + def.Long,
		Desc: def.Desc,
	}
	if def.Short != "" {
		f.Short = "-" + def.Short
	}

	if def.Placeholder != nil {
		f.Placeholder = def.Placeholder.Name
	}

	return f
}

func shouldSkipTargFlag(f flags.Def, filter TargFlagFilter) bool {
	if f.RootOnly && !filter.IsRoot {
		return true
	}

	switch f.Long {
	case "completion":
		return filter.DisableCompletion
	case "help":
		return filter.DisableHelp
	case "timeout":
		return filter.DisableTimeout
	default:
		return false
	}
}
