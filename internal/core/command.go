// Package core provides the internal implementation of targ command parsing and execution.
package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/toejough/targ/internal/help"
	internalsh "github.com/toejough/targ/internal/sh"
)

// DepGroupDisplay holds display information for a dependency group.
type DepGroupDisplay struct {
	Names []string
	Mode  string
}

// GroupLike is implemented by types that represent target groups.
type GroupLike interface {
	GetName() string
	GetMembers() []any
}

// TargetConfigLike is implemented by types that provide target configuration.
type TargetConfigLike interface {
	// GetConfig returns (watchPatterns, cachePatterns, watchDisabled, cacheDisabled)
	GetConfig() ([]string, []string, bool, bool)
}

// TargetLike is implemented by types that represent build targets.
type TargetLike interface {
	Fn() any
	GetName() string
	GetDescription() string
}

// AppendBuiltinExamples adds built-in examples after custom examples.
func AppendBuiltinExamples(custom ...Example) []Example {
	return append(custom, BuiltinExamples()...)
}

// BuiltinExamples returns the default targ examples (completion setup, chaining).
func BuiltinExamples() []Example {
	return []Example{
		completionExampleWithGetenv(os.Getenv),
		chainExample(nil),
	}
}

// EmptyExamples returns an empty slice to disable examples in help.
func EmptyExamples() []Example {
	return []Example{}
}

// PrependBuiltinExamples adds built-in examples before custom examples.
func PrependBuiltinExamples(custom ...Example) []Example {
	return append(BuiltinExamples(), custom...)
}

// unexported constants.
const (
	flagPlaceholder = "[flag]"
	usageLineWidth  = 80
)

// unexported variables.
var (
	errExpectedFunc          = errors.New("expected func")
	errFieldNotExported      = errors.New("field must be exported")
	errFuncMustAcceptContext = errors.New("function command must accept context.Context")
	errFuncMustReturnError   = errors.New("function command must return only error")
	errFuncTooManyInputs     = errors.New("function command must be niladic or accept context")
	errLongFlagFormat        = errors.New("long flags must use --")
	errMissingRequiredFlag   = errors.New("missing required flag")
	errNilFunctionCommand    = errors.New("nil function command")
	errNilTarget             = errors.New("nil target")
	errShortFlagGroupNotBool = errors.New("short flag group must contain only boolean flags")
	errStructNotSupported    = errors.New(
		"struct commands are not supported; use targ.Targ(fn) instead",
	)
	errTagOptsInvalidInput     = errors.New("TagOptions must accept (string, TagOptions)")
	errTagOptsInvalidOutput    = errors.New("TagOptions must return (TagOptions, error)")
	errTagOptsInvalidSignature = errors.New(
		"TagOptions must accept (string, TagOptions) and return (TagOptions, error)",
	)
	errTargetInvalidFnType       = errors.New("Target.Fn() must be func or string")
	errUnableToDetermineFuncName = errors.New("unable to determine function name")
	errUnrecognizedTagKeys       = errors.New("unrecognized struct tag keys")
	// shellVarPattern matches $var or ${var} style variables in shell commands.
	shellVarPattern = regexp.MustCompile(`\$\{?([a-zA-Z_][a-zA-Z0-9_]*)\}?`)
)

type commandInstance struct {
	node  *commandNode
	value reflect.Value
}

type commandNode struct {
	Name        string
	Type        reflect.Type
	Value       reflect.Value // The struct instance
	Func        reflect.Value // Niladic function target
	Parent      *commandNode
	Subcommands map[string]*commandNode
	RunMethod   reflect.Value
	Description string
	SourceFile  string // Source file path for build tool mode

	// Shell command support
	ShellCommand string   // Shell command string (e.g., "kubectl apply -n $namespace")
	ShellVars    []string // Variable names extracted from ShellCommand (lowercase)

	// Target configuration for conflict detection with CLI overrides
	WatchPatterns []string
	CachePatterns []string
	WatchDisabled bool
	CacheDisabled bool

	// Execution configuration for help display
	DepGroups       []DepGroupDisplay // Dependency groups with modes
	Timeout         time.Duration     // execution timeout
	Times           int               // number of times to run
	Retry           bool              // continue on failure
	BackoffInitial  time.Duration     // initial backoff delay
	BackoffMultiply float64           // backoff multiplier

	// Target reference for dep execution
	Target *Target
}

// --- Execution ---

func (n *commandNode) execute(ctx context.Context, args []string, opts RunOptions) error {
	_, err := n.executeWithParents(ctx, args, nil, map[string]bool{}, false, opts)
	return err
}

func (n *commandNode) executeWithParents(
	ctx context.Context,
	args []string,
	parents []commandInstance,
	visited map[string]bool,
	explicit bool,
	opts RunOptions,
) ([]string, error) {
	if opts.HelpOnly {
		w := opts.Stdout
		printCommandHelp(w, n, opts)
		_, _ = fmt.Fprintln(w)
	}

	if n.Func.IsValid() {
		return executeFunctionWithParents(ctx, args, n, parents, visited, explicit, opts)
	}

	if n.ShellCommand != "" {
		return executeShellCommand(ctx, args, n, parents, visited, explicit, opts)
	}

	// Handle groups: nodes with Subcommands but no Func
	// Check before deps-only targets since groups may also have Target set for attribution
	if len(n.Subcommands) > 0 {
		return executeGroupWithParents(ctx, args, n, parents, visited, opts)
	}

	// Handle deps-only targets: no Func/ShellCommand but has Target with deps
	if n.Target != nil {
		return executeDepsOnlyTarget(ctx, args, n, opts)
	}

	// Fallback: node with no execution capability
	return args, nil
}

type flagHelp struct {
	Name        string
	Short       string
	Usage       string
	Options     string
	Placeholder string
	Required    bool
	Inherited   bool
}

type flagSpec struct {
	value          reflect.Value
	name           string
	short          string
	env            string
	defaultValue   *string
	required       bool
	defaultApplied bool
	envApplied     bool
}

type positionalHelp struct {
	Name        string
	Placeholder string
	Required    bool
}

// shellArgParseResult holds the result of parsing shell command arguments.
type shellArgParseResult struct {
	varValues     map[string]string
	remaining     []string
	helpRequested bool
}

func appendDepsLine(lines []string, node *commandNode) []string {
	if len(node.DepGroups) == 0 {
		return lines
	}

	// Single group — use original format for backward compatibility
	if len(node.DepGroups) == 1 {
		g := node.DepGroups[0]

		mode := g.Mode
		if mode == "" {
			mode = DepModeSerial.String()
		}

		return append(lines, fmt.Sprintf("Deps: %s (%s)", strings.Join(g.Names, ", "), mode))
	}

	// Multiple groups — use arrow separator
	var parts []string

	for _, g := range node.DepGroups {
		part := strings.Join(g.Names, ", ")
		if g.Mode == DepModeParallel.String() {
			part += " (parallel)"
		}

		parts = append(parts, part)
	}

	return append(lines, "Deps: "+strings.Join(parts, " → "))
}

func appendPatternsLine(lines []string, label string, patterns []string) []string {
	if len(patterns) == 0 {
		return lines
	}

	return append(lines, label+": "+strings.Join(patterns, ", "))
}

func appendRetryLine(lines []string, node *commandNode) []string {
	if !node.Retry {
		return lines
	}

	if node.BackoffInitial > 0 {
		return append(
			lines,
			fmt.Sprintf(
				"Retry: yes (backoff: %s × %.1f)",
				node.BackoffInitial,
				node.BackoffMultiply,
			),
		)
	}

	return append(lines, "Retry: yes")
}

func appendTimeoutLine(lines []string, node *commandNode) []string {
	if node.Timeout <= 0 {
		return lines
	}

	return append(lines, fmt.Sprintf("Timeout: %s", node.Timeout))
}

func appendTimesLine(lines []string, node *commandNode) []string {
	if node.Times <= 0 {
		return lines
	}

	return append(lines, fmt.Sprintf("Times: %d", node.Times))
}

func applyDefaultsAndEnv(specs []*flagSpec, visited map[string]bool) error {
	for _, spec := range specs {
		if flagVisited(spec, visited) {
			continue
		}

		if spec.env != "" {
			if value := os.Getenv(spec.env); value != "" {
				err := setFieldFromString(spec.value, value)
				if err != nil {
					return fmt.Errorf("invalid value for env %s: %w", spec.env, err)
				}

				spec.envApplied = true

				continue
			}
		}

		if spec.defaultValue != nil {
			err := setFieldFromString(spec.value, *spec.defaultValue)
			if err != nil {
				return fmt.Errorf("invalid default for --%s: %w", spec.name, err)
			}

			spec.defaultApplied = true
		}
	}

	return nil
}

func applyTagOptionsOverride(
	inst reflect.Value,
	field reflect.StructField,
	opts TagOptions,
) (TagOptions, error) {
	// Get TagOptions method - inline of tagOptionsMethod
	var method reflect.Value

	if inst.IsValid() {
		target := inst
		if inst.Kind() != reflect.Ptr && inst.CanAddr() {
			target = inst.Addr()
		}

		method = target.MethodByName("TagOptions")
	}

	if !method.IsValid() {
		return opts, nil
	}

	err := validateTagOptionsSignature(method)
	if err != nil {
		return opts, err
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(field.Name),
		reflect.ValueOf(opts),
	})

	// Results are validated by validateTagOptionsSignature to have exactly 2 elements
	// with types (TagOptions, error). This check satisfies the nil-checker.
	if len(results) < 2 { //nolint:mnd // 2 is the validated return count
		return opts, nil
	}

	if !results[1].IsNil() {
		// Type assertion is safe because validateTagOptionsSignature ensures error type
		//nolint:forcetypeassert // validated by validateTagOptionsSignature
		return opts, results[1].Interface().(error)
	}

	// Type assertion is safe because validateTagOptionsSignature ensures TagOptions type
	//nolint:forcetypeassert // validated by validateTagOptionsSignature
	return results[0].Interface().(TagOptions), nil
}

// applyTagPart applies a single tag part to options and returns whether it was recognized.
// Returns true if the part was recognized (e.g., "required", "name=value"), false otherwise.
func applyTagPart(opts *TagOptions, p string) bool {
	setters := []struct {
		prefix string
		apply  func(opts *TagOptions, val string)
	}{
		{"name=", func(opts *TagOptions, val string) { opts.Name = val }},
		{"short=", func(opts *TagOptions, val string) { opts.Short = val }},
		{"env=", func(opts *TagOptions, val string) { opts.Env = val }},
		{"default=", func(opts *TagOptions, val string) { opts.Default = &val }},
		{"enum=", func(opts *TagOptions, val string) { opts.Enum = val }},
		{"placeholder=", func(opts *TagOptions, val string) { opts.Placeholder = val }},
		{"desc=", func(opts *TagOptions, val string) { opts.Desc = val }},
		{"description=", func(opts *TagOptions, val string) { opts.Desc = val }},
	}

	for _, setter := range setters {
		if after, ok := strings.CutPrefix(p, setter.prefix); ok {
			setter.apply(opts, after)
			return true
		}
	}

	if p == "required" {
		opts.Required = true
		return true
	}

	// Part is not recognized
	return false
}

func buildExecInfo(lines []string) *help.ExecutionInfo {
	if len(lines) == 0 {
		return nil
	}

	execInfo := &help.ExecutionInfo{}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "Deps:"):
			execInfo.Deps = strings.TrimPrefix(line, "Deps: ")
		case strings.HasPrefix(line, "Cache:"):
			execInfo.CachePatterns = strings.TrimPrefix(line, "Cache: ")
		case strings.HasPrefix(line, "Watch:"):
			execInfo.WatchPatterns = strings.TrimPrefix(line, "Watch: ")
		case strings.HasPrefix(line, "Timeout:"):
			execInfo.Timeout = strings.TrimPrefix(line, "Timeout: ")
		case strings.HasPrefix(line, "Times:"):
			execInfo.Times = strings.TrimPrefix(line, "Times: ")
		case strings.HasPrefix(line, "Retry:"):
			execInfo.Retry = strings.TrimPrefix(line, "Retry: ")
		}
	}

	return execInfo
}

// --- Help output ---

// buildCommandPath builds the full command path from root to this node.

func buildFlagMaps(specs []*flagSpec) (shortInfo, longInfo map[string]bool) {
	shortInfo = map[string]bool{}
	longInfo = map[string]bool{}

	for _, spec := range specs {
		longInfo[spec.name] = true

		if spec.short != "" {
			shortInfo[spec.short] = spec.value.Kind() == reflect.Bool
		}
	}

	return shortInfo, longInfo
}

// buildPositionalParts builds usage parts for positional arguments.
func buildPositionalParts(node *commandNode) ([]string, error) {
	positionals, err := collectPositionalHelp(node)
	if err != nil {
		return nil, err
	}

	parts := make([]string, 0, len(positionals))

	for _, item := range positionals {
		name := positionalDisplayName(item)
		if item.Required {
			parts = append(parts, name)
		} else {
			parts = append(parts, fmt.Sprintf("[%s...]", name))
		}
	}

	return parts, nil
}

// buildShortToLongMap builds a map from short flag letters to long flag names.
func buildShortToLongMap(vars []string) map[string]string {
	result := make(map[string]string)
	usedShorts := make(map[rune]bool)

	for _, varName := range vars {
		if len(varName) > 0 {
			firstRune := rune(varName[0])
			if !usedShorts[firstRune] {
				result[string(firstRune)] = varName
				usedShorts[firstRune] = true
			}
		}
	}

	return result
}

// buildUsageParts builds the usage parts for a command.
func buildUsageParts(node *commandNode) ([]string, error) {
	parts := []string{node.Name}

	flags, err := collectFlagHelpChain(node)
	if err != nil {
		return nil, err
	}

	// Show required flags inline, count optional flags
	hasOptionalFlags := false

	for _, item := range flags {
		if item.Required {
			parts = append(parts, formatFlagUsage(item))
		} else {
			hasOptionalFlags = true
		}
	}

	// If there are subcommands, show chaining pattern
	if len(node.Subcommands) > 0 {
		return append(parts, "<subcommand>...", "[^", "<command>...]"), nil
	}

	positionalParts, err := buildPositionalParts(node)
	if err != nil {
		return nil, err
	}

	parts = append(parts, positionalParts...)

	// Show [flags...] at end if there are optional flags
	if hasOptionalFlags {
		parts = append(parts, "[flags...]")
	}

	return parts, nil
}

// callFunctionWithArgs calls a function with context and/or struct args.
// / Handles: func(), func(ctx), func(args), func(ctx, args)
//
//nolint:cyclop // Complex function signature handling requires many branches
func callFunctionWithArgs(ctx context.Context, fn, argsInst reflect.Value) error {
	if !fn.IsValid() || (fn.Kind() == reflect.Func && fn.IsNil()) {
		return errNilFunctionCommand
	}

	ft := fn.Type()

	var callArgs []reflect.Value

	for i := range ft.NumIn() {
		paramType := ft.In(i)

		// Check if this param is context.Context
		if paramType == reflect.TypeFor[context.Context]() {
			callArgs = append(callArgs, reflect.ValueOf(ctx))
			continue
		}

		// Otherwise it's the args struct - use the parsed instance
		//nolint:gocritic // if-else chain is clearer than switch for type checks
		if argsInst.IsValid() && argsInst.Type() == paramType {
			callArgs = append(callArgs, argsInst)
		} else if argsInst.IsValid() && argsInst.CanAddr() && argsInst.Addr().Type() == paramType {
			// Handle pointer to struct
			callArgs = append(callArgs, argsInst.Addr())
		} else {
			// Create zero value if no instance
			callArgs = append(callArgs, reflect.Zero(paramType))
		}
	}

	results := fn.Call(callArgs)

	// Check for error return
	if len(results) > 0 {
		last := results[len(results)-1]
		if last.Type().Implements(reflect.TypeFor[error]()) && !last.IsNil() {
			if err, ok := last.Interface().(error); ok {
				return err
			}
		}
	}

	return nil
}

// --- Utilities ---

func camelToKebab(s string) string {
	var result strings.Builder

	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			// Insert hyphen if previous is lowercase (e.g., fooBar -> foo-bar)
			// OR if we're at the start of a new word after an acronym (e.g., APIServer -> api-server)
			if unicode.IsLower(prev) || (i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
				result.WriteRune('-')
			}
		}

		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

// chainExample returns an example showing command chaining.
// If nodes are provided, uses actual command names.
//
//nolint:cyclop // Traversing command tree structure requires many branches
func chainExample(nodes []*commandNode) Example {
	// Check if any nodes have subcommands (nested structure)
	var (
		groupWithSub *commandNode
		subName      string
		otherCmd     string
	)

	for _, node := range nodes {
		if len(node.Subcommands) > 0 && groupWithSub == nil {
			groupWithSub = node
			// Get first subcommand name
			for name := range node.Subcommands {
				subName = name
				break
			}
		} else if groupWithSub != nil && otherCmd == "" {
			otherCmd = node.Name
		}
	}

	// If we have nested groups, show ^ example
	if groupWithSub != nil && otherCmd != "" {
		return Example{
			Title: "Chain nested commands (^ returns to root)",
			Code:  fmt.Sprintf("targ %s %s ^ %s", groupWithSub.Name, subName, otherCmd),
		}
	}

	// Flat structure - show simple chaining
	var names []string

	seenSources := make(map[string]bool)

	for _, node := range nodes {
		source := node.SourceFile
		if !seenSources[source] && len(names) < 2 {
			names = append(names, node.Name)
			seenSources[source] = true
		}
	}

	if len(names) < 2 { //nolint:mnd // Need at least 2 commands for a chaining example
		names = []string{"build", "test"}
	}

	return Example{
		Title: "Run multiple commands",
		Code:  fmt.Sprintf("targ %s %s", names[0], names[1]),
	}
}

func checkRequiredFlags(specs []*flagSpec, visited map[string]bool) error {
	for _, spec := range specs {
		if !spec.required {
			continue
		}

		if flagVisited(spec, visited) || spec.defaultApplied || spec.envApplied {
			continue
		}

		display := "--" + spec.name
		if spec.short != "" {
			display = fmt.Sprintf("--%s, -%s", spec.name, spec.short)
		}

		return fmt.Errorf("%w: %s", errMissingRequiredFlag, display)
	}

	return nil
}

// checkUnknownFlags validates that remaining args don't contain unrecognized flags.
// This ensures flag validation happens before shell command execution.
func checkUnknownFlags(remaining []string) error {
	for _, arg := range remaining {
		if after, ok := strings.CutPrefix(arg, "--"); ok {
			flagName := after
			if idx := strings.Index(flagName, "="); idx >= 0 {
				flagName = flagName[:idx]
			}

			return fmt.Errorf("%w: --%s", errFlagNotDefined, flagName)
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Short flag like -x or -x=value
			flagName := arg[1:2] // First char after -

			return fmt.Errorf("%w: -%s", errFlagNotDefined, flagName)
		}
	}

	return nil
}

func collectFlagHelp(node *commandNode) ([]flagHelp, error) {
	// Shell command targets: generate flags from $var placeholders
	if len(node.ShellVars) > 0 {
		return shellVarFlagHelp(node.ShellVars), nil
	}

	if node.Type == nil {
		return nil, nil
	}

	typ := node.Type
	inst := reflect.New(node.Type).Elem()

	var flags []flagHelp

	for i := range typ.NumField() {
		field := typ.Field(i)

		help, ok, err := flagHelpForField(inst, field)
		if err != nil {
			return nil, err
		}

		if ok {
			flags = append(flags, help)
		}
	}

	return flags, nil
}

func collectFlagHelpChain(node *commandNode) ([]flagHelp, error) {
	chain := nodeChain(node)

	var flags []flagHelp

	for i, current := range chain {
		inherited := i < len(chain)-1

		items, err := collectFlagHelp(current)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			item.Inherited = inherited
			flags = append(flags, item)
		}
	}

	return flags, nil
}

func collectHelpSubcommands(node *commandNode) []help.Subcommand {
	if len(node.Subcommands) == 0 {
		return nil
	}

	var helpSubs []help.Subcommand

	for _, name := range sortedKeys(node.Subcommands) {
		if sub := node.Subcommands[name]; sub != nil {
			helpSubs = append(helpSubs, help.Subcommand{Name: name, Desc: sub.Description})
		}
	}

	return helpSubs
}

func collectPositionalHelp(node *commandNode) ([]positionalHelp, error) {
	if node.Type == nil {
		return nil, nil
	}

	typ := node.Type
	inst := reflect.New(node.Type).Elem()

	positionals := make([]positionalHelp, 0, typ.NumField())

	for i := range typ.NumField() {
		field := typ.Field(i)

		opts, err := tagOptionsForField(inst, field)
		if err != nil {
			return nil, err
		}

		if opts.Kind != TagKindPositional {
			continue
		}

		if !field.IsExported() {
			return nil, fmt.Errorf("%w: %s", errFieldNotExported, field.Name)
		}

		placeholder := opts.Placeholder
		if opts.Enum != "" {
			placeholder = fmt.Sprintf("{%s}", opts.Enum)
		}

		positionals = append(positionals, positionalHelp{
			Name:        opts.Name,
			Placeholder: placeholder,
			Required:    opts.Required,
		})
	}

	return positionals, nil
}

// completionExampleWithGetenv returns a shell-specific completion setup example using injected getenv.
func completionExampleWithGetenv(getenv func(string) string) Example {
	shell := detectCurrentShell(getenv)

	var code string

	switch shell {
	case zshShell:
		code = "source <(targ --completion)"
	case fishShell:
		code = "targ --completion | source"
	default:
		code = "eval \"$(targ --completion)\""
	}

	return Example{
		Title: "Enable shell completion",
		Code:  code,
	}
}

func convertExamples(examples []Example) []help.Example {
	if examples == nil {
		return nil
	}

	helpExamples := make([]help.Example, 0, len(examples))
	for _, e := range examples {
		helpExamples = append(helpExamples, help.Example{Title: e.Title, Code: e.Code})
	}

	return helpExamples
}

func convertFlagHelps(flagHelps []flagHelp) []help.Flag {
	helpFlags := make([]help.Flag, 0, len(flagHelps))

	for _, f := range flagHelps {
		hf := help.Flag{
			Long:        "--" + f.Name,
			Desc:        f.Usage,
			Placeholder: f.Placeholder,
			Required:    f.Required,
		}
		if f.Short != "" {
			hf.Short = "-" + f.Short
		}

		helpFlags = append(helpFlags, hf)
	}

	return helpFlags
}

// detectCurrentShell returns the name of the current shell using the provided getenv function.
// Defaults to "bash" if SHELL is not set, since bash syntax is the fallback.
func detectCurrentShell(getenv func(string) string) string {
	shell := getenv("SHELL")
	if shell == "" {
		return bashShell
	}

	return filepath.Base(shell)
}

func detectTagKind(opts *TagOptions, tag, fieldName string) {
	if strings.Contains(tag, "positional") {
		opts.Kind = TagKindPositional
		opts.Name = fieldName
	}
}

// executeDepsOnlyTarget handles targets that have no function but run dependencies.
func executeDepsOnlyTarget(
	ctx context.Context,
	args []string,
	node *commandNode,
	opts RunOptions,
) ([]string, error) {
	// Skip execution in help-only mode
	if opts.HelpOnly {
		return args, nil
	}

	// Run dependencies
	if len(node.Target.depGroups) > 0 {
		err := node.Target.runDeps(ctx)
		if err != nil {
			return nil, err
		}
	}

	return args, nil
}

func executeFunctionWithParents(
	ctx context.Context,
	args []string,
	node *commandNode,
	parents []commandInstance,
	visited map[string]bool,
	explicit bool,
	opts RunOptions,
) ([]string, error) {
	// Create instance for current node if it has a Type (struct arg)
	inst := nodeInstance(node)

	// Build chain including current node for flag collection
	chain := slices.Concat(parents, []commandInstance{{node: node, value: inst}})

	specs, _, err := collectFlagSpecs(chain)
	if err != nil {
		return nil, err
	}

	result, err := parseCommandArgs(
		node,
		inst,
		chain,
		args,
		visited,
		explicit,
		true,
		false,
	)
	if err != nil {
		return nil, err
	}
	// In help-only mode, skip validation and execution
	if opts.HelpOnly {
		return result.remaining, nil
	}

	err = applyDefaultsAndEnv(specs, visited)
	if err != nil {
		return nil, err
	}

	err = checkRequiredFlags(specs, visited)
	if err != nil {
		return nil, err
	}

	err = runTargetWithOverrides(ctx, node, inst, opts)
	if err != nil {
		return nil, err
	}

	return result.remaining, nil
}

// executeGroupWithParents handles navigation through groups (nodes with Subcommands but no Func).
func executeGroupWithParents(
	ctx context.Context,
	args []string,
	node *commandNode,
	parents []commandInstance,
	visited map[string]bool,
	opts RunOptions,
) ([]string, error) {
	if len(args) == 0 {
		// No subcommand specified - return remaining args
		return args, nil
	}

	subName := args[0]

	// Check for glob patterns in subcommand name
	if isGlobPatternCmd(subName) {
		matches := findMatchingSubcommands(node, subName)
		if len(matches) == 0 {
			// No matches - return all args
			return args, nil
		}

		// Execute each matching subcommand
		chain := slices.Concat(parents, []commandInstance{{node: node}})

		for _, sub := range matches {
			_, err := sub.executeWithParents(ctx, nil, chain, visited, true, opts)
			if err != nil {
				return nil, err
			}
		}

		return args[1:], nil
	}

	// Look for matching subcommand
	for name, sub := range node.Subcommands {
		if strings.EqualFold(name, subName) {
			// Found matching subcommand - execute it
			chain := slices.Concat(parents, []commandInstance{{node: node}})
			return sub.executeWithParents(ctx, args[1:], chain, visited, true, opts)
		}
	}

	// No matching subcommand - return all args
	return args, nil
}

// executeShellCommand handles execution of shell command targets with $var substitution.
func executeShellCommand(
	ctx context.Context,
	args []string,
	node *commandNode,
	_ []commandInstance, // parents - not used for shell commands
	_ map[string]bool, // visited - not used for shell commands
	_ bool, // explicit - not used for shell commands
	opts RunOptions,
) ([]string, error) {
	parsed := parseShellCommandArgs(args, node.ShellVars)

	if parsed.helpRequested {
		printCommandHelp(opts.Stdout, node, opts)
		return nil, nil
	}

	if opts.HelpOnly {
		return parsed.remaining, nil
	}

	err := validateShellVars(parsed.varValues, node.ShellVars)
	if err != nil {
		return nil, err
	}

	// Check for unknown flags before execution
	err = checkUnknownFlags(parsed.remaining)
	if err != nil {
		return nil, err
	}

	config := TargetConfig{
		WatchPatterns: node.WatchPatterns,
		CachePatterns: node.CachePatterns,
		WatchDisabled: node.WatchDisabled,
		CacheDisabled: node.CacheDisabled,
	}

	err = ExecuteWithOverrides(ctx, opts.Overrides, config, func() error {
		return runShellWithVars(ctx, node.ShellCommand, parsed.varValues, opts.ShellRunner)
	})
	if err != nil {
		return nil, err
	}

	return parsed.remaining, nil
}

// executionInfoLines builds the lines for execution info display.
func executionInfoLines(node *commandNode) []string {
	lines := make([]string, 0)

	lines = appendDepsLine(lines, node)
	lines = appendPatternsLine(lines, "Cache", node.CachePatterns)
	lines = appendPatternsLine(lines, "Watch", node.WatchPatterns)
	lines = appendTimeoutLine(lines, node)
	lines = appendTimesLine(lines, node)
	lines = appendRetryLine(lines, node)

	return lines
}

func expandFlagGroup(arg, group string, shortInfo map[string]bool) ([]string, error) {
	allBool := true
	unknown := false

	for _, ch := range group {
		isBool, ok := shortInfo[string(ch)]
		if !ok {
			unknown = true

			break
		}

		if !isBool {
			allBool = false

			break
		}
	}

	if unknown {
		return []string{arg}, nil
	}

	if !allBool {
		return nil, fmt.Errorf("%w: %q", errShortFlagGroupNotBool, arg)
	}

	expanded := make([]string, 0, len(group))
	for _, ch := range group {
		expanded = append(expanded, "-"+string(ch))
	}

	return expanded, nil
}

func expandShortFlagGroups(args []string, specs []*flagSpec) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}

	shortInfo, longInfo := buildFlagMaps(specs)

	var expanded []string

	for _, arg := range args {
		if skipShortExpansion(arg, longInfo) {
			expanded = append(expanded, arg)
			continue
		}

		group := strings.TrimPrefix(arg, "-")

		expandedFlags, err := expandFlagGroup(arg, group, shortInfo)
		if err != nil {
			return nil, err
		}

		expanded = append(expanded, expandedFlags...)
	}

	return expanded, nil
}

// extractShellVars extracts unique variable names from a shell command.
// Returns lowercase variable names in order of first occurrence.
func extractShellVars(cmd string) []string {
	matches := shellVarPattern.FindAllStringSubmatch(cmd, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)

	var vars []string

	for _, match := range matches {
		if len(match) < 2 { //nolint:mnd // regex submatch: [full, capture]
			continue
		}

		varName := strings.ToLower(match[1])
		if !seen[varName] {
			seen[varName] = true

			vars = append(vars, varName)
		}
	}

	return vars
}

// findMatchingSubcommands finds all subcommands matching a glob pattern.
func findMatchingSubcommands(node *commandNode, pattern string) []*commandNode {
	matches := make([]*commandNode, 0)

	for name, sub := range node.Subcommands {
		if matchesGlobCmd(name, pattern) {
			matches = append(matches, sub)
		}
	}

	return matches
}

func flagHelpForField(inst reflect.Value, field reflect.StructField) (flagHelp, bool, error) {
	opts, err := tagOptionsForField(inst, field)
	if err != nil {
		return flagHelp{}, false, err
	}

	if opts.Kind != TagKindFlag {
		return flagHelp{}, false, nil
	}

	if !field.IsExported() {
		return flagHelp{}, false, fmt.Errorf("%w: %s", errFieldNotExported, field.Name)
	}

	placeholder := resolvePlaceholder(opts, field.Type.Kind())

	return flagHelp{
		Name:        opts.Name,
		Short:       opts.Short,
		Usage:       opts.Desc,
		Options:     "",
		Placeholder: placeholder,
		Required:    opts.Required,
	}, true, nil
}

// --- Flag handling ---

func flagSpecForField(
	inst reflect.Value,
	field reflect.StructField,
	fieldVal reflect.Value,
) (*flagSpec, bool, error) {
	opts, err := tagOptionsForField(inst, field)
	if err != nil {
		return nil, false, err
	}

	if !field.IsExported() {
		return nil, false, fmt.Errorf("%w: %s", errFieldNotExported, field.Name)
	}

	if opts.Kind != TagKindFlag {
		return nil, false, nil
	}

	return &flagSpec{
		value:        fieldVal,
		name:         opts.Name,
		short:        opts.Short,
		env:          opts.Env,
		defaultValue: opts.Default,
		required:     opts.Required,
	}, true, nil
}

func flagVisited(spec *flagSpec, visited map[string]bool) bool {
	if visited[spec.name] {
		return true
	}

	if spec.short != "" && visited[spec.short] {
		return true
	}

	return false
}

func formatFlagUsage(item flagHelp) string {
	name := "--" + item.Name
	if item.Short != "" {
		name = fmt.Sprintf("{-%s|--%s}", item.Short, item.Name)
	}

	if item.Placeholder != "" && item.Placeholder != flagPlaceholder {
		name = fmt.Sprintf("%s %s", name, item.Placeholder)
	}

	return name
}

// formatSourceAttribution returns the source attribution string for a target.
// Returns empty string if showAttribution is false (backwards compat).

// Local target

// Remote target

// funcSourceFile returns the source file path for a function.
// Callers must ensure v is a valid, non-nil function value.
func funcSourceFile(v reflect.Value) string {
	fn := runtime.FuncForPC(v.Pointer())
	file, _ := fn.FileLine(v.Pointer())

	return file
}

func functionName(v reflect.Value) string {
	fn := runtime.FuncForPC(v.Pointer())
	if fn == nil {
		return ""
	}

	name := fn.Name()
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}

	name = strings.TrimSuffix(name, "-fm")

	return name
}

// groupNodesBySource groups nodes by their source file, preserving order.
func groupNodesBySource(nodes []*commandNode, opts RunOptions) []struct {
	source string
	nodes  []*commandNode
} {
	// Use a slice to preserve order, map for lookup
	groups := make([]struct {
		source string
		nodes  []*commandNode
	}, 0, len(nodes))
	sourceIndex := make(map[string]int)

	getwd := optsGetwd(opts)

	for _, node := range nodes {
		source := relativeSourcePathWithGetwd(node.SourceFile, getwd)
		if source == "" {
			source = "(unknown)"
		}

		if idx, ok := sourceIndex[source]; ok {
			groups[idx].nodes = append(groups[idx].nodes, node)
		} else {
			sourceIndex[source] = len(groups)
			groups = append(groups, struct {
				source string
				nodes  []*commandNode
			}{source: source, nodes: []*commandNode{node}})
		}
	}

	return groups
}

// isGlobPatternCmd checks if a string contains glob metacharacters.
func isGlobPatternCmd(s string) bool {
	return strings.Contains(s, "*")
}

// isShellVar checks if a flag name matches one of the shell variables.
func isShellVar(name string, vars []string) bool {
	for _, v := range vars {
		if strings.EqualFold(name, v) {
			return true
		}
	}

	return false
}

// matchesGlobCmd checks if a name matches a glob pattern.
// Supports * (any characters) at start, end, or both.
func matchesGlobCmd(name, pattern string) bool {
	if pattern == "*" || pattern == "**" {
		return true
	}

	// Handle patterns like "test-*" or "*-unit"
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		middle := pattern[1 : len(pattern)-1]
		return strings.Contains(strings.ToLower(name), strings.ToLower(middle))
	}

	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix))
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix))
	}

	// No wildcards - exact match
	return strings.EqualFold(name, pattern)
}

func nodeChain(node *commandNode) []*commandNode {
	if node == nil {
		return nil
	}

	chain := make([]*commandNode, 0)
	for current := node; current != nil; current = current.Parent {
		chain = append(chain, current)
	}

	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return chain
}

func nodeHasAddressableValue(node *commandNode) bool {
	return node != nil && node.Value.IsValid() &&
		node.Value.Kind() == reflect.Struct && node.Value.CanAddr()
}

func nodeInstance(node *commandNode) reflect.Value {
	if nodeHasAddressableValue(node) {
		return node.Value
	}

	if node == nil || node.Type == nil {
		return reflect.Value{}
	}

	return reflect.New(node.Type).Elem()
}

// optsGetwd returns the Getwd function from opts.
func optsGetwd(opts RunOptions) func() (string, error) {
	return opts.Getwd
}

func parseFunc(v reflect.Value) (*commandNode, error) {
	typ := v.Type()
	if typ.Kind() != reflect.Func {
		return nil, fmt.Errorf("%w, got %v", errExpectedFunc, typ.Kind())
	}

	err := validateFuncType(typ)
	if err != nil {
		return nil, err
	}

	name := functionName(v)
	if name == "" {
		return nil, errUnableToDetermineFuncName
	}

	return &commandNode{
		Name:        camelToKebab(name),
		Func:        v,
		Subcommands: make(map[string]*commandNode),
		SourceFile:  funcSourceFile(v),
	}, nil
}

// parseGroupLike creates a commandNode from a GroupLike (targ.Group).
func parseGroupLike(group GroupLike) (*commandNode, error) {
	node := &commandNode{
		Name:        group.GetName(),
		Subcommands: make(map[string]*commandNode),
	}

	// Preserve source attribution if this group has GetSource method
	if sourceAware, ok := group.(interface{ GetSource() string }); ok {
		// Create a lightweight wrapper to hold source info
		// This allows groups to participate in source attribution display
		groupTarget := &Target{
			name:      group.GetName(),
			sourcePkg: sourceAware.GetSource(),
		}
		node.Target = groupTarget
	}

	members := group.GetMembers()

	for idx, member := range members {
		subNode, err := parseTarget(member) // Recursive
		if err != nil {
			return nil, fmt.Errorf("group member %d: %w", idx, err)
		}

		subNode.Parent = node
		node.Subcommands[subNode.Name] = subNode
	}

	return node, nil
}

// parseShellCommandArgs parses flags from args to get variable values for shell vars.
func parseShellCommandArgs(args, shellVars []string) shellArgParseResult {
	varValues := make(map[string]string)
	remaining := make([]string, 0, len(args))
	shortToLong := buildShortToLongMap(shellVars)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--help" || arg == "-h" {
			return shellArgParseResult{helpRequested: true, remaining: args[i+1:]}
		}

		if skip, handled := parseShellLongFlag(
			arg,
			args,
			i,
			shellVars,
			varValues,
			&remaining,
		); handled {
			i += skip
			continue
		}

		if skip, handled := parseShellShortFlag(arg, args, i, shortToLong, varValues); handled {
			i += skip
			continue
		}

		remaining = append(remaining, arg)
	}

	return shellArgParseResult{varValues: varValues, remaining: remaining}
}

// parseShellLongFlag parses a long flag (--flag or --flag=value).
// Returns skip count and whether the arg was handled.
func parseShellLongFlag(
	arg string,
	args []string,
	idx int,
	shellVars []string,
	varValues map[string]string,
	remaining *[]string,
) (skip int, handled bool) {
	after, ok := strings.CutPrefix(arg, "--")
	if !ok {
		return 0, false
	}

	flagName := after
	value := ""

	if eqIdx := strings.Index(flagName, "="); eqIdx != -1 {
		value = flagName[eqIdx+1:]
		flagName = flagName[:eqIdx]
	} else if idx+1 < len(args) && !strings.HasPrefix(args[idx+1], "-") {
		skip = 1
		value = args[idx+1]
	}

	if isShellVar(flagName, shellVars) {
		varValues[strings.ToLower(flagName)] = value
	} else {
		*remaining = append(*remaining, arg)
	}

	return skip, true
}

// parseShellShortFlag parses a short flag (-f).
// Returns skip count and whether the arg was handled.
func parseShellShortFlag(
	arg string,
	args []string,
	idx int,
	shortToLong map[string]string,
	varValues map[string]string,
) (skip int, handled bool) {
	if !strings.HasPrefix(arg, "-") || len(arg) <= 1 {
		return 0, false
	}

	shortFlag := string(arg[1])
	longName, ok := shortToLong[shortFlag]

	if !ok {
		return 0, false
	}

	value := ""

	if idx+1 < len(args) && !strings.HasPrefix(args[idx+1], "-") {
		skip = 1
		value = args[idx+1]
	}

	varValues[longName] = value

	return skip, true
}

// parseTagParts parses tag parts and returns an error if unknown keys are found.
func parseTagParts(opts *TagOptions, tag string) error {
	parts := strings.SplitSeq(tag, ",")

	var unknownKeys []string

	for p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Skip keywords handled elsewhere
		if p == "flag" || p == "positional" {
			continue
		}

		// Check if this part is recognized
		if !applyTagPart(opts, p) {
			unknownKeys = append(unknownKeys, p)
		}
	}

	if len(unknownKeys) > 0 {
		return fmt.Errorf(
			"%w: %s (valid: name, short, env, default, enum, placeholder, desc, description, required, positional, flag)",
			errUnrecognizedTagKeys, strings.Join(unknownKeys, ", "),
		)
	}

	return nil
}

// --- Parsing targets ---

func parseTarget(t any) (*commandNode, error) {
	if t == nil {
		return nil, errNilTarget
	}

	// Check for TargetLike (targ.Target)
	if tl, ok := t.(TargetLike); ok {
		return parseTargetLike(tl)
	}

	// Check for GroupLike (targ.Group)
	if gl, ok := t.(GroupLike); ok {
		return parseGroupLike(gl)
	}

	v := reflect.ValueOf(t)
	if v.Kind() == reflect.Func {
		return parseFunc(v)
	}

	// Struct commands are no longer supported
	return nil, errStructNotSupported
}

// parseTargetLike creates a commandNode from a TargetLike (targ.Target).
func parseTargetLike(target TargetLike) (*commandNode, error) {
	fn := target.Fn()

	// Handle string (shell command), function, or deps-only (nil fn)
	var node *commandNode

	var err error

	switch v := fn.(type) {
	case nil:
		// Deps-only target - create node with just a name, no function
		node = &commandNode{
			Name: target.GetName(),
		}
	case string:
		node = parseTargetLikeString(target, v)
	default:
		node, err = parseTargetLikeFunc(target, fn)
		if err != nil {
			return nil, err
		}
	}

	// Extract target config if available (for conflict detection)
	if configTarget, ok := target.(TargetConfigLike); ok {
		watch, cache, watchDis, cacheDis := configTarget.GetConfig()
		node.WatchPatterns = watch
		node.CachePatterns = cache
		node.WatchDisabled = watchDis
		node.CacheDisabled = cacheDis
	}

	// Extract execution config if available (for help display and dep execution)
	if execTarget, ok := target.(TargetExecutionLike); ok {
		for _, g := range execTarget.GetDepGroups() {
			var names []string
			for _, d := range g.Targets {
				names = append(names, d.GetName())
			}

			node.DepGroups = append(node.DepGroups, DepGroupDisplay{
				Names: names,
				Mode:  g.Mode.String(),
			})
		}

		node.Timeout = execTarget.GetTimeout()
		node.Times = execTarget.GetTimes()
		node.Retry = execTarget.GetRetry()
		node.BackoffInitial, node.BackoffMultiply = execTarget.GetBackoff()
	}

	// Store Target reference for dep execution and resolve source file
	if t, ok := target.(*Target); ok {
		node.Target = t
		resolveTargetSource(node, t)
	}

	return node, nil
}

// parseTargetLikeFunc creates a commandNode for a function target.
//
//nolint:cyclop // Parsing different function signatures requires many type checks
func parseTargetLikeFunc(target TargetLike, fn any) (*commandNode, error) {
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		return nil, errTargetInvalidFnType
	}

	ft := fv.Type()

	// Validate return type
	if ft.NumOut() > 1 || (ft.NumOut() == 1 && !ft.Out(0).Implements(reflect.TypeFor[error]())) {
		return nil, errFuncMustReturnError
	}

	// Get function name
	name := functionName(fv)
	if name == "" {
		return nil, errUnableToDetermineFuncName
	}

	node := &commandNode{
		Name:        camelToKebab(name),
		Func:        fv,
		Subcommands: make(map[string]*commandNode),
		SourceFile:  funcSourceFile(fv),
	}

	// Check for struct argument (for flag parsing)
	for i := range ft.NumIn() {
		paramType := ft.In(i)
		if paramType == reflect.TypeFor[context.Context]() {
			continue
		}
		// Non-context parameter - if it's a struct, set up node.Type for flag parsing
		if paramType.Kind() == reflect.Struct {
			node.Type = paramType
		}

		break // Only handle first non-context parameter
	}

	// Override with Target metadata if set
	if name := target.GetName(); name != "" {
		node.Name = name
	}

	if desc := target.GetDescription(); desc != "" {
		node.Description = desc
	}

	return node, nil
}

// parseTargetLikeString creates a commandNode for a shell command target.
func parseTargetLikeString(target TargetLike, cmd string) *commandNode {
	node := &commandNode{
		Name:         target.GetName(),
		Description:  target.GetDescription(),
		Subcommands:  make(map[string]*commandNode),
		ShellCommand: cmd,
	}

	// If no name set, use the first word of the command
	if node.Name == "" && cmd != "" {
		parts := strings.Fields(cmd)
		if len(parts) > 0 {
			node.Name = parts[0]
		}
	}

	// Extract variable names from the command (e.g., $namespace, ${file})
	node.ShellVars = extractShellVars(cmd)

	return node
}

func positionalDisplayName(item positionalHelp) string {
	if item.Placeholder != "" {
		return item.Placeholder
	}

	if item.Name != "" {
		return item.Name
	}

	return "ARG"
}

func printCommandHelp(w io.Writer, node *commandNode, opts RunOptions) {
	usageParts, err := buildUsageParts(node)
	if err != nil {
		_, _ = fmt.Fprintf(w, "Error: %v\n", err)
		return
	}

	usageParts = append([]string{opts.BinaryName, "[targ flags...]"}, usageParts...)

	flagItems, err := collectFlagHelp(node)
	if err != nil {
		_, _ = fmt.Fprintf(w, "Error: %v\n", err)
		return
	}

	help.WriteTargetHelp(w, help.TargetHelpOpts{
		BinaryName:    opts.BinaryName,
		Name:          node.Name,
		Description:   node.Description,
		SourceFile:    relativeSourcePathWithGetwd(node.SourceFile, optsGetwd(opts)),
		ShellCommand:  node.ShellCommand,
		Usage:         strings.Join(usageParts, " "),
		Flags:         convertFlagHelps(flagItems),
		Subcommands:   collectHelpSubcommands(node),
		ExecutionInfo: buildExecInfo(executionInfoLines(node)),
		Examples:      convertExamples(opts.Examples),
		MoreInfoText:  resolveMoreInfoText(opts),
		Filter: help.TargFlagFilter{
			IsRoot:            false,
			BinaryMode:        opts.BinaryMode,
			DisableCompletion: opts.DisableCompletion,
			DisableHelp:       opts.DisableHelp,
			DisableTimeout:    opts.DisableTimeout,
		},
	})
}

func printUsage(w io.Writer, nodes []*commandNode, opts RunOptions) {
	// Convert node groups to help.CommandGroup
	groups := groupNodesBySource(nodes, opts)

	cmdGroups := make([]help.CommandGroup, 0, len(groups))

	for _, g := range groups {
		cmds := make([]help.Command, 0, len(g.nodes))
		for _, n := range g.nodes {
			cmds = append(cmds, help.Command{Name: n.Name, Desc: n.Description})
		}

		cmdGroups = append(cmdGroups, help.CommandGroup{Source: g.source, Commands: cmds})
	}

	// Convert examples (let WriteRootHelp auto-generate if not provided)
	var helpExamples []help.Example
	if opts.Examples != nil {
		helpExamples = make([]help.Example, 0, len(opts.Examples))
		for _, e := range opts.Examples {
			helpExamples = append(helpExamples, help.Example{Title: e.Title, Code: e.Code})
		}
	}

	// Get more info text
	moreInfo := opts.MoreInfoText
	if moreInfo == "" {
		if url := opts.RepoURL; url != "" {
			moreInfo = url
		} else if url := DetectRepoURL(); url != "" {
			moreInfo = url
		}
	}

	help.WriteRootHelp(w, help.RootHelpOpts{
		BinaryName:           opts.BinaryName,
		Description:          opts.Description,
		CommandGroups:        cmdGroups,
		DeregisteredPackages: opts.DeregisteredPackages,
		Examples:             helpExamples,
		MoreInfoText:         moreInfo,
		Filter: help.TargFlagFilter{
			IsRoot:            true,
			BinaryMode:        opts.BinaryMode,
			DisableCompletion: opts.DisableCompletion,
			DisableHelp:       opts.DisableHelp,
			DisableTimeout:    opts.DisableTimeout,
		},
	})
}

// relativeSourcePathWithGetwd returns a relative path if possible, otherwise the absolute path.
func relativeSourcePathWithGetwd(absPath string, getwd func() (string, error)) string {
	if absPath == "" {
		return ""
	}

	cwd, err := getwd()
	if err != nil {
		return absPath
	}

	relPath, err := filepath.Rel(cwd, absPath)
	if err != nil {
		return absPath
	}

	return relPath
}

func resolveMoreInfoText(opts RunOptions) string {
	if opts.MoreInfoText != "" {
		return opts.MoreInfoText
	}

	if opts.RepoURL != "" {
		return opts.RepoURL
	}

	return DetectRepoURL()
}

func resolvePlaceholder(opts TagOptions, kind reflect.Kind) string {
	if opts.Enum != "" {
		return fmt.Sprintf("{%s}", opts.Enum)
	}

	if opts.Placeholder != "" {
		return opts.Placeholder
	}

	switch kind { //nolint:exhaustive // only common types have placeholders
	case reflect.String:
		return "<string>"
	case reflect.Int:
		return "<int>"
	case reflect.Bool:
		return "[flag]"
	default:
		return ""
	}
}

// resolveTargetSource sets the display source file on a commandNode.
// Priority: sourcePkg (remote targets) > sourceFile (local string/deps-only) > funcSourceFile (already set).
func resolveTargetSource(node *commandNode, t *Target) {
	if src := t.GetSource(); src != "" {
		node.SourceFile = src
	} else if sf := t.GetSourceFile(); sf != "" && node.SourceFile == "" {
		node.SourceFile = sf
	}
}

// runShellWithVars substitutes variables and executes a shell command.
// If runner is nil, uses the default sh -c execution.
func runShellWithVars(
	ctx context.Context,
	cmd string,
	vars map[string]string,
	runner func(ctx context.Context, cmd string) error,
) error {
	// Substitute $var and ${var} patterns
	substituted := shellVarPattern.ReplaceAllStringFunc(cmd, func(match string) string {
		submatch := shellVarPattern.FindStringSubmatch(match)
		if len(submatch) < 2 { //nolint:mnd // regex submatch: [full, capture]
			return match
		}

		varName := strings.ToLower(submatch[1])
		if val, ok := vars[varName]; ok {
			return val
		}

		return match
	})

	// Execute via injected runner or default sh -c
	var err error
	if runner != nil {
		err = runner(ctx, substituted)
	} else {
		err = internalsh.RunContextWithIO(ctx, nil, "sh", []string{"-c", substituted})
	}

	if err != nil {
		return fmt.Errorf("shell command failed: %w", err)
	}

	return nil
}

// runTargetWithOverrides runs the target's dependencies and function with runtime overrides.
//
//nolint:nestif // nested conditions reflect inherent dep-mode override logic
func runTargetWithOverrides(
	ctx context.Context,
	node *commandNode,
	inst reflect.Value,
	opts RunOptions,
) error {
	// Run dependencies first (if Target with deps is available)
	if node.Target != nil && len(node.Target.depGroups) > 0 {
		target := node.Target

		// Apply --dep-mode override: flatten all groups into one
		if opts.Overrides.DepMode != "" {
			var mode DepMode
			if opts.Overrides.DepMode == depModeParallelStr {
				mode = DepModeParallel
			}

			allDeps := target.GetDeps()
			if len(allDeps) > 0 {
				target.depGroups = []depGroup{{targets: allDeps, mode: mode}}
			}
		}

		err := target.runDeps(ctx)
		if err != nil {
			return err
		}
	}

	// Deps-only targets have no function to execute
	if !node.Func.IsValid() {
		return nil
	}

	// Execute with runtime overrides (times, retry, watch, cache, etc.)
	config := TargetConfig{
		WatchPatterns: node.WatchPatterns,
		CachePatterns: node.CachePatterns,
		WatchDisabled: node.WatchDisabled,
		CacheDisabled: node.CacheDisabled,
	}

	return ExecuteWithOverrides(ctx, opts.Overrides, config, func() error {
		return callFunctionWithArgs(ctx, node.Func, inst)
	})
}

// shellVarFlagHelp generates synthetic flag help for shell command variables.
func shellVarFlagHelp(vars []string) []flagHelp {
	flags := make([]flagHelp, 0, len(vars))
	usedShorts := make(map[rune]bool)

	for _, varName := range vars {
		flag := flagHelp{
			Name:        varName,
			Placeholder: "VALUE",
			Required:    true, // All shell vars are required
		}

		// Assign short flag from first letter if not already used
		if len(varName) > 0 {
			firstRune := rune(varName[0])
			if !usedShorts[firstRune] {
				flag.Short = string(firstRune)
				usedShorts[firstRune] = true
			}
		}

		flags = append(flags, flag)
	}

	return flags
}

func skipShortExpansion(arg string, longInfo map[string]bool) bool {
	if arg == "--" || strings.HasPrefix(arg, "--") {
		return true
	}

	if len(arg) <= 2 || !strings.HasPrefix(arg, "-") {
		return true
	}

	if strings.Contains(arg, "=") {
		return true
	}

	group := strings.TrimPrefix(arg, "-")

	return len(group) <= 1 || longInfo[group]
}

// sortedKeys returns sorted keys from a command map.
func sortedKeys(m map[string]*commandNode) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

func tagOptionsForField(inst reflect.Value, field reflect.StructField) (TagOptions, error) {
	tag := field.Tag.Get("targ")

	opts := TagOptions{
		Kind: TagKindFlag,
		Name: toKebabCase(field.Name),
	}
	if strings.TrimSpace(tag) == "" {
		overridden, err := applyTagOptionsOverride(inst, field, opts)
		if err != nil {
			return TagOptions{}, err
		}

		return overridden, nil
	}

	detectTagKind(&opts, tag, field.Name)

	err := parseTagParts(&opts, tag)
	if err != nil {
		return TagOptions{}, fmt.Errorf("invalid tag for field %s: %w", field.Name, err)
	}

	overridden, err := applyTagOptionsOverride(inst, field, opts)
	if err != nil {
		return TagOptions{}, err
	}

	return overridden, nil
}

// --- Tag options ---

// toKebabCase converts CamelCase to kebab-case.
// Examples: MinRetrievals → min-retrievals, HTTPPort → http-port
func toKebabCase(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder

	const estimatedHyphens = 3

	result.Grow(len(s) + estimatedHyphens)

	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			// Insert hyphen before uppercase letter if:
			// 1. Previous char was lowercase (camelCase boundary)
			// 2. Previous char was uppercase AND next char is lowercase (acronym end)
			if unicode.IsLower(rune(s[i-1])) {
				result.WriteRune('-')
			} else if i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
				result.WriteRune('-')
			}
		}

		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

func validateFuncType(typ reflect.Type) error {
	if typ.NumIn() > 1 {
		return errFuncTooManyInputs
	}

	if typ.NumIn() == 1 && typ.In(0) != reflect.TypeFor[context.Context]() {
		return errFuncMustAcceptContext
	}

	if typ.NumOut() == 0 {
		return nil
	}

	if typ.NumOut() == 1 && typ.Out(0).Implements(reflect.TypeFor[error]()) {
		return nil
	}

	return errFuncMustReturnError
}

func validateLongFlagArgs(args []string, longNames map[string]bool) error {
	for _, arg := range args {
		if arg == "--" {
			return nil
		}

		if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") || len(arg) <= 2 {
			continue
		}

		name := strings.TrimPrefix(arg, "-")
		if idx := strings.Index(name, "="); idx >= 0 {
			name = name[:idx]
		}

		if len(name) <= 1 {
			continue
		}

		if longNames[name] {
			return fmt.Errorf("%w%s (got -%s)", errLongFlagFormat, name, name)
		}
	}

	return nil
}

// validateShellVars checks that all required shell variables are provided.
func validateShellVars(varValues map[string]string, requiredVars []string) error {
	for _, varName := range requiredVars {
		if _, ok := varValues[varName]; !ok {
			return fmt.Errorf("%w: --%s", errMissingRequiredFlag, varName)
		}
	}

	return nil
}

func validateTagOptionsSignature(method reflect.Value) error {
	mtype := method.Type()
	if mtype.NumIn() != 2 || mtype.NumOut() != 2 {
		return errTagOptsInvalidSignature
	}

	if mtype.In(0).Kind() != reflect.String || mtype.In(1) != reflect.TypeFor[TagOptions]() {
		return errTagOptsInvalidInput
	}

	if mtype.Out(0) != reflect.TypeFor[TagOptions]() ||
		!mtype.Out(1).Implements(reflect.TypeFor[error]()) {
		return errTagOptsInvalidOutput
	}

	return nil
}

// writeWrappedUsage writes a usage line with wrapping at word boundaries.

// Build lines by adding parts until we exceed the target width
