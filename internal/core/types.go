package core

import (
	"io"
	"time"
)

// Exported constants.
const (
	TagKindFlag       TagKind = "flag"
	TagKindPositional TagKind = "positional"
	TagKindUnknown    TagKind = "unknown"
)

// Example represents a usage example shown in help text.
type Example struct {
	Title string // e.g., "Enable shell completion"
	Code  string // e.g., "eval \"$(targ --completion)\""
}

// ExecuteResult contains the result of executing a command.
type ExecuteResult struct {
	Output string
}

// Interleaved wraps a value to be parsed from interleaved positional arguments.
type Interleaved[T any] struct {
	Value    T
	Position int
}

// RunOptions configures command execution behavior.
type RunOptions struct {
	AllowDefault      bool
	DisableHelp       bool
	DisableTimeout    bool
	DisableCompletion bool
	HelpOnly          bool // Internal: set when --help is detected, skips execution

	// Description is shown at the top of help output (before Usage).
	// Only shown for top-level --help, not when a specific command is requested.
	Description string

	// RepoURL is the repository URL shown in help output "More info" section.
	// If empty, targ attempts to detect it from .git/config.
	RepoURL string

	// MoreInfoText overrides the default "More info" section in help output.
	// If set, this text is shown instead of the auto-generated repo URL line.
	MoreInfoText string

	// Examples to show in help output. If nil, built-in examples are shown.
	// Use EmptyExamples() to disable examples entirely.
	// Use AppendBuiltinExamples() to add custom examples alongside built-ins.
	Examples []Example

	// Overrides are runtime flags that override Target compile-time settings.
	// Internal: populated by extracting --times, --watch, etc. from args.
	Overrides RuntimeOverrides

	// Stdout is the writer for help/usage output.
	// Internal: set by the executor to the env's stdout.
	Stdout io.Writer
}

// TagKind represents the type of a struct tag (flag, positional, subcommand).
type TagKind string

// TagOptions holds parsed struct tag options for CLI argument handling.
type TagOptions struct {
	Kind        TagKind
	Name        string
	Short       string
	Desc        string
	Env         string
	Default     *string
	Enum        string
	Placeholder string
	Required    bool
}

// TargetExecutionLike is implemented by types that provide execution configuration.
type TargetExecutionLike interface {
	GetDeps() []*Target
	GetDepMode() DepMode
	GetTimeout() time.Duration
	GetTimes() int
	GetRetry() bool
	GetBackoff() (time.Duration, float64)
}

// unexported constants.
const (
	disabledSentinel = "__targ_disabled__"
)
