// Package flags provides the centralized flag registry for targ.
// All help, completion, and detection logic derives from this registry.
package flags

import "strings"

// Def describes a CLI flag for help, completion, and detection.
type Def struct {
	Long        string       // without "--", e.g. "timeout"
	Short       string       // without "-", e.g. "p" (empty if none)
	Desc        string       // help text
	Placeholder *Placeholder // value placeholder with format info (nil if TakesValue is false)
	TakesValue  bool         // consumes next arg as value
	RootOnly    bool         // only valid before any command
	Hidden      bool         // excluded from help/completion (deprecated aliases)
	Removed     string       // non-empty = removed flag, value is error message
}

// All returns the canonical registry of targ flags.
//
//nolint:funlen // Registry literals are clearer as one list.
func All() []Def {
	cmd := placeholderCmd()
	dir := placeholderDir()
	duration := placeholderDuration()
	durationMult := placeholderDurationMult()
	glob := placeholderGlob()
	mode := placeholderMode()
	n := placeholderN()
	shell := placeholderShell()

	return []Def{
		// Runtime flags (handled by core during target execution)
		{
			Long:        "completion",
			Desc:        "Generate shell completion script",
			Placeholder: &shell,
			TakesValue:  true,
			RootOnly:    true,
		},
		{Long: "help", Short: "h", Desc: "Show help"},
		{
			Long:        "source",
			Short:       "s",
			Desc:        "Use targ files from specified directory",
			Placeholder: &dir,
			TakesValue:  true,
			RootOnly:    true,
		},
		{
			Long:        "timeout",
			Desc:        "Set execution timeout",
			Placeholder: &duration,
			TakesValue:  true,
		},
		{Long: "parallel", Short: "p", Desc: "Run multiple targets concurrently"},
		{
			Long:        "times",
			Desc:        "Run the command n times",
			Placeholder: &n,
			TakesValue:  true,
		},
		{Long: "retry", Desc: "Continue on failure"},
		{
			Long:        "backoff",
			Desc:        "Exponential backoff",
			Placeholder: &durationMult,
			TakesValue:  true,
		},
		{
			Long:        "watch",
			Desc:        "Re-run on file changes (repeatable)",
			Placeholder: &glob,
			TakesValue:  true,
		},
		{
			Long:        "cache",
			Desc:        "Skip if files unchanged (repeatable)",
			Placeholder: &glob,
			TakesValue:  true,
		},
		{
			Long:        "while",
			Desc:        "Run while shell command succeeds",
			Placeholder: &cmd,
			TakesValue:  true,
		},
		{
			Long:        "dep-mode",
			Desc:        "Dependency mode",
			Placeholder: &mode,
			TakesValue:  true,
		},
		{Long: "no-binary-cache", Desc: "Disable binary caching", RootOnly: true},

		// Early flags (handled by runner before binary compilation)
		// These trigger special handling and have dedicated help pages (use --help --<flag>)
		{Long: "create", Desc: "Create a new target", RootOnly: true},
		{Long: "sync", Desc: "Sync targets from a remote package", RootOnly: true},
		{Long: "to-func", Desc: "Convert string target to function", RootOnly: true},
		{Long: "to-string", Desc: "Convert function target to string", RootOnly: true},

		// Deprecated/removed
		{Long: "no-cache", Desc: "Deprecated: use --no-binary-cache", Hidden: true},
		{Long: "init", Removed: "flag has been removed; use --create instead"},
		{Long: "alias", Removed: "flag has been removed; use --create instead"},
		{Long: "move", Removed: "flag has been removed; use --create instead"},
	}
}

// BooleanFlags returns map of --long and -short flags that don't take values.
func BooleanFlags() map[string]bool {
	return booleanFlags(All())
}

// Find returns the flag def matching arg (e.g. "--create", "-p"), or nil.
func Find(arg string) *Def {
	return findInDefs(All(), arg)
}

// GlobalFlags returns --long names of flags valid at any command level.
func GlobalFlags() []string {
	return globalFlags(All())
}

// RootOnlyFlags returns --long names of flags only valid at root.
func RootOnlyFlags() []string {
	return rootOnlyFlags(All())
}

// VisibleFlags returns all non-hidden, non-removed flags.
func VisibleFlags() []Def {
	return visibleFlags(All())
}

// WithValues returns map of --long flags that consume next arg.
func WithValues() map[string]bool {
	return withValues(All())
}

func booleanFlags(defs []Def) map[string]bool {
	m := make(map[string]bool)

	for _, f := range defs {
		if !f.TakesValue && f.Removed == "" {
			m["--"+f.Long] = true
			if f.Short != "" {
				m["-"+f.Short] = true
			}
		}
	}

	return m
}

func findInDefs(defs []Def, arg string) *Def {
	if after, ok := strings.CutPrefix(arg, "--"); ok {
		// Strip =value suffix for --flag=value forms.
		name := after
		if idx := strings.Index(name, "="); idx >= 0 {
			name = name[:idx]
		}

		for i := range defs {
			if defs[i].Long == name {
				return &defs[i]
			}
		}

		return nil
	}

	if after, ok := strings.CutPrefix(arg, "-"); ok && len(after) == 1 {
		for i := range defs {
			if defs[i].Short == after {
				return &defs[i]
			}
		}
	}

	return nil
}

func globalFlags(defs []Def) []string {
	var out []string

	for _, f := range defs {
		if !f.RootOnly && !f.Hidden && f.Removed == "" {
			out = append(out, "--"+f.Long)
		}
	}

	return out
}

func rootOnlyFlags(defs []Def) []string {
	var out []string

	for _, f := range defs {
		if f.RootOnly && !f.Hidden && f.Removed == "" {
			out = append(out, "--"+f.Long)
		}
	}

	return out
}

func visibleFlags(defs []Def) []Def {
	var out []Def

	for _, f := range defs {
		if !f.Hidden && f.Removed == "" {
			out = append(out, f)
		}
	}

	return out
}

func withValues(defs []Def) map[string]bool {
	m := make(map[string]bool)

	for _, f := range defs {
		if f.TakesValue && f.Removed == "" {
			m["--"+f.Long] = true
		}
	}

	return m
}
