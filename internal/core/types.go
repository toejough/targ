package core

// Exported constants.
const (
	TagKindFlag       TagKind = "flag"
	TagKindPositional TagKind = "positional"
	TagKindSubcommand TagKind = "subcommand"
	TagKindUnknown    TagKind = "unknown"
)

// Interleaved wraps a value with its parse position for tracking flag ordering.
// Use []Interleaved[T] when you need to know the relative order of flags
// across multiple slice fields (e.g., interleaved --include and --exclude).
type Interleaved[T any] struct {
	Value    T
	Position int
}

// RunOptions controls runtime behavior for RunWithOptions.
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
}

// TagKind identifies the type of a struct field in command parsing.
type TagKind string

// TagOptions contains parsed tag options for a struct field.
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
