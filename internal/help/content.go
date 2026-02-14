package help

// Command represents a command entry in help output.
type Command struct {
	Name string
	Desc string
}

// CommandGroup represents a group of commands from the same source file.
type CommandGroup struct {
	Source   string
	Commands []Command
}

// ContentBuilder holds all help content before rendering.
// Fields are unexported; use builder methods to populate.
type ContentBuilder struct {
	commandName   string
	description   string
	usage         string
	sourceFile    string
	shellCommand  string
	positionals   []Positional
	globalFlags   []Flag
	rootOnlyFlags []Flag
	commandFlags  []Flag
	values        []Value
	formats       []Format
	subcommands   []Subcommand
	commandGroups []CommandGroup
	executionInfo *ExecutionInfo
	examples      []Example
	moreInfoText  string
	isRoot        bool
	binaryMode    bool // true for compiled binary mode, false for targ CLI mode
	examplesSet   bool // distinguishes nil (use defaults) from empty (no examples)
}

// Example represents a usage example with title and code.
type Example struct {
	Title string
	Code  string
}

// ExecutionInfo represents execution configuration for a target.
type ExecutionInfo struct {
	Deps          string // "build, test (serial)"
	CachePatterns string // "*.go, **/*.mod"
	WatchPatterns string // "*.go"
	Timeout       string // "30s"
	Times         string // "3"
	Retry         string // "yes (backoff: 1s × 2.0)"
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

// Positional represents a positional argument in command usage.
type Positional struct {
	Name        string
	Placeholder string
	Required    bool
}

// Subcommand represents a subcommand entry in help output.
type Subcommand struct {
	Name string
	Desc string
}

// Value represents a value type description (e.g., shell values for --completion).
type Value struct {
	Name string
	Desc string
}
