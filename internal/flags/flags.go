// Package flags provides the centralized flag registry for targ.
// All help, completion, and detection logic derives from this registry.
package flags

import "strings"

// All is the complete registry of targ flags.
//
//nolint:gochecknoglobals // Read-only flag registry, initialized once.
var All = []Def{
		// Runtime flags (handled by core during target execution)
		{
			Long:        "completion",
			Desc:        "Generate shell completion script",
			Placeholder: &PlaceholderShell,
			TakesValue:  true,
			RootOnly:    true,
		},
		{Long: "help", Short: "h", Desc: "Show help"},
		{
			Long:        "source",
			Short:       "s",
			Desc:        "Use targ files from specified directory",
			Placeholder: &PlaceholderDir,
			TakesValue:  true,
			RootOnly:    true,
		},
		{
			Long:        "timeout",
			Desc:        "Set execution timeout",
			Placeholder: &PlaceholderDuration,
			TakesValue:  true,
		},
		{Long: "parallel", Short: "p", Desc: "Run multiple targets concurrently"},
		{
			Long:        "times",
			Desc:        "Run the command n times",
			Placeholder: &PlaceholderN,
			TakesValue:  true,
		},
		{Long: "retry", Desc: "Continue on failure"},
		{
			Long:        "backoff",
			Desc:        "Exponential backoff",
			Placeholder: &PlaceholderDurationMult,
			TakesValue:  true,
		},
		{
			Long:        "watch",
			Desc:        "Re-run on file changes (repeatable)",
			Placeholder: &PlaceholderGlob,
			TakesValue:  true,
		},
		{
			Long:        "cache",
			Desc:        "Skip if files unchanged (repeatable)",
			Placeholder: &PlaceholderGlob,
			TakesValue:  true,
		},
		{
			Long:        "while",
			Desc:        "Run while shell command succeeds",
			Placeholder: &PlaceholderCmd,
			TakesValue:  true,
		},
		{
			Long:        "dep-mode",
			Desc:        "Dependency mode",
			Placeholder: &PlaceholderMode,
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

// BooleanFlags returns map of --long and -short flags that don't take values.
func BooleanFlags() map[string]bool {
	m := make(map[string]bool)

	for _, f := range All {
		if !f.TakesValue && f.Removed == "" {
			m["--"+f.Long] = true
			if f.Short != "" {
				m["-"+f.Short] = true
			}
		}
	}

	return m
}

// Find returns the flag def matching arg (e.g. "--create", "-p"), or nil.
func Find(arg string) *Def {
	if after, ok := strings.CutPrefix(arg, "--"); ok {
		// Strip =value suffix for --flag=value forms.
		name := after
		if idx := strings.Index(name, "="); idx >= 0 {
			name = name[:idx]
		}

		for i := range All {
			if All[i].Long == name {
				return &All[i]
			}
		}

		return nil
	}

	if after, ok := strings.CutPrefix(arg, "-"); ok && len(after) == 1 {
		for i := range All {
			if All[i].Short == after {
				return &All[i]
			}
		}
	}

	return nil
}

// GlobalFlags returns --long names of flags valid at any command level.
func GlobalFlags() []string {
	var out []string

	for _, f := range All {
		if !f.RootOnly && !f.Hidden && f.Removed == "" {
			out = append(out, "--"+f.Long)
		}
	}

	return out
}

// RootOnlyFlags returns --long names of flags only valid at root.
func RootOnlyFlags() []string {
	var out []string

	for _, f := range All {
		if f.RootOnly && !f.Hidden && f.Removed == "" {
			out = append(out, "--"+f.Long)
		}
	}

	return out
}

// VisibleFlags returns all non-hidden, non-removed flags.
func VisibleFlags() []Def {
	var out []Def

	for _, f := range All {
		if !f.Hidden && f.Removed == "" {
			out = append(out, f)
		}
	}

	return out
}

// WithValues returns map of --long flags that consume next arg.
func WithValues() map[string]bool {
	m := make(map[string]bool)

	for _, f := range All {
		if f.TakesValue && f.Removed == "" {
			m["--"+f.Long] = true
		}
	}

	return m
}
