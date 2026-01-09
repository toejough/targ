package core

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
}

// TagKind identifies the type of a struct field in command parsing.
type TagKind string

const (
	TagKindUnknown    TagKind = "unknown"
	TagKindFlag       TagKind = "flag"
	TagKindPositional TagKind = "positional"
	TagKindSubcommand TagKind = "subcommand"
)

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
