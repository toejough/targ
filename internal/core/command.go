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
	"runtime"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// GroupLike is implemented by targ.Group for discovery integration.
type GroupLike interface {
	GetName() string
	GetMembers() []any
}

// TargetLike is implemented by targ.Target for discovery integration.
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
		completionExample(),
		chainExample(nil),
	}
}

// builtinExamplesForNodes returns examples using actual command names.
func builtinExamplesForNodes(nodes []*commandNode) []Example {
	return []Example{
		completionExample(),
		chainExample(nodes),
	}
}

// chainExample returns an example showing command chaining.
// If nodes are provided, uses actual command names.
func chainExample(nodes []*commandNode) Example {
	// Get up to 2 command names from different source files
	var names []string
	seenSources := make(map[string]bool)

	for _, node := range nodes {
		source := getNodeSourceFile(node)
		if !seenSources[source] && len(names) < 2 {
			names = append(names, node.Name)
			seenSources[source] = true
		}
	}

	// Fall back to generic if not enough commands
	if len(names) < 2 {
		names = []string{"build", "test"}
	}

	return Example{
		Title: "Chain commands (^ separates independent commands)",
		Code:  fmt.Sprintf("targ %s ^ %s", names[0], names[1]),
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
	flagPlaceholder    = "[flag]"
	tagOptsReturnCount = 2
	usageLineWidth     = 80
)

// unexported variables.
var (
	errExpectedFunc     = errors.New("expected func")
	errExpectedStruct   = errors.New("expected struct")
	errFieldNotExported = errors.New("field must be exported")
	errFieldNotZero     = errors.New(
		"must be zero value; use default tags instead of prefilled fields",
	)
	errFuncMustAcceptContext   = errors.New("function command must accept context.Context")
	errFuncMustReturnError     = errors.New("function command must return only error")
	errFuncTooManyInputs       = errors.New("function command must be niladic or accept context")
	errLongFlagFormat          = errors.New("long flags must use --")
	errMethodMustAcceptContext = errors.New("must accept context.Context")
	errMethodMustReturnError   = errors.New("must return only error")
	errMethodTooManyInputs     = errors.New("must accept context.Context or no args")
	errMissingRequiredFlag     = errors.New("missing required flag")
	errNilFunctionCommand      = errors.New("nil function command")
	errNilPointerTarget        = errors.New("nil pointer target")
	errNilTarget               = errors.New("nil target")
	errShortFlagGroupNotBool   = errors.New("short flag group must contain only boolean flags")
	errSubcommandNil           = errors.New("subcommand is nil")
	errSubcommandPrefilled     = errors.New(
		"must not prefill subcommand; use default tags instead",
	)
	errTagOptsInvalidInput     = errors.New("TagOptions must accept (string, TagOptions)")
	errTagOptsInvalidOutput    = errors.New("TagOptions must return (TagOptions, error)")
	errTagOptsInvalidSignature = errors.New(
		"TagOptions must accept (string, TagOptions) and return (TagOptions, error)",
	)
	errTagOptsNonErrorType       = errors.New("TagOptions method returned non-error type")
	errTagOptsWrongType          = errors.New("TagOptions method returned wrong type")
	errTagOptsWrongValueCount    = errors.New("TagOptions method returned wrong number of values")
	errTargetInvalidFnType       = errors.New("Target.Fn() must be func or string")
	errUnableToDetermineFuncName = errors.New("unable to determine function name")
	errUnsupportedFieldType      = errors.New("unsupported subcommand field type")
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
}

// --- Execution ---

func (n *commandNode) execute(ctx context.Context, args []string, opts RunOptions) error {
	_, err := n.executeWithParents(ctx, args, nil, map[string]bool{}, false, opts)
	return err
}

// executeSiblings handles implicit sibling resolution for remaining args.
func (n *commandNode) executeSiblings(
	ctx context.Context,
	inst reflect.Value,
	chain []commandInstance,
	remaining []string,
	visited map[string]bool,
	opts RunOptions,
) ([]string, error) {
	for len(remaining) > 0 {
		if remaining[0] == "^" {
			return remaining[1:], nil
		}

		sibling := n.findSibling(remaining[0])
		if sibling == nil {
			break
		}

		err := assignSubcommandField(n, inst, sibling.Name, sibling)
		if err != nil {
			return nil, err
		}

		remaining, err = sibling.executeWithParents(ctx, remaining[1:], chain, visited, true, opts)
		if err != nil {
			return nil, err
		}
	}

	return remaining, nil
}

// executeStructCommand handles execution of struct-based commands.
func (n *commandNode) executeStructCommand(
	ctx context.Context,
	args []string,
	parents []commandInstance,
	visited map[string]bool,
	explicit bool,
	opts RunOptions,
) ([]string, error) {
	inst, err := nodeInstance(n)
	if err != nil {
		return nil, err
	}

	chain := slices.Concat(parents, []commandInstance{{node: n, value: inst}})

	specs, _, err := collectFlagSpecs(chain)
	if err != nil {
		return nil, err
	}

	result, err := parseCommandArgs(n, inst, chain, args, visited, explicit, true, false)
	if err != nil {
		return nil, err
	}

	if result.subcommand != nil {
		return n.executeSubcommand(ctx, inst, chain, result, visited, opts)
	}

	if opts.HelpOnly {
		return result.remaining, nil
	}

	return n.runCommandWithHooks(ctx, inst, chain, specs, visited, result.remaining)
}

// executeSubcommand handles subcommand dispatch and sibling resolution.
func (n *commandNode) executeSubcommand(
	ctx context.Context,
	inst reflect.Value,
	chain []commandInstance,
	result parseResult,
	visited map[string]bool,
	opts RunOptions,
) ([]string, error) {
	err := assignSubcommandField(n, inst, result.subcommand.Name, result.subcommand)
	if err != nil {
		return nil, err
	}

	remaining, err := result.subcommand.executeWithParents(
		ctx,
		result.remaining,
		chain,
		visited,
		true,
		opts,
	)
	if err != nil {
		return nil, err
	}

	return n.executeSiblings(ctx, inst, chain, remaining, visited, opts)
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
		printCommandHelp(n, opts)
		fmt.Println()
	}

	if remaining, done := handleHelpFlag(n, args, opts); done {
		return remaining, nil
	}

	ctx, args, cancel, err := applyTimeout(ctx, args, opts)
	if err != nil {
		return nil, err
	}

	if cancel != nil {
		defer cancel()
	}

	if n.Func.IsValid() {
		return executeFunctionWithParents(ctx, args, n, parents, visited, explicit, opts)
	}

	return n.executeStructCommand(ctx, args, parents, visited, explicit, opts)
}

// findSibling finds a sibling subcommand by name (case-insensitive).
func (n *commandNode) findSibling(name string) *commandNode {
	for subName, sub := range n.Subcommands {
		if strings.EqualFold(subName, name) {
			return sub
		}
	}

	return nil
}

// runCommandWithHooks executes the command with before/after hooks.
func (n *commandNode) runCommandWithHooks(
	ctx context.Context,
	inst reflect.Value,
	chain []commandInstance,
	specs []*flagSpec,
	visited map[string]bool,
	remaining []string,
) ([]string, error) {
	err := applyDefaultsAndEnv(specs, visited)
	if err != nil {
		return nil, err
	}

	err = checkRequiredFlags(specs, visited)
	if err != nil {
		return nil, err
	}

	err = runPersistentHooks(ctx, chain, "PersistentBefore")
	if err != nil {
		return nil, err
	}

	err = runCommand(ctx, n, inst, nil, 0)
	if err != nil {
		return nil, err
	}

	err = runPersistentHooks(ctx, reverseChain(chain), "PersistentAfter")
	if err != nil {
		return nil, err
	}

	return remaining, nil
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

// applyNameAndDescription overrides node name and description from methods.
func applyNameAndDescription(node *commandNode, v reflect.Value, typ reflect.Type) {
	if cmdName := getCommandName(v, typ); cmdName != "" {
		node.Name = cmdName
	}

	if desc := getDescription(v, typ); desc != "" {
		node.Description = desc
	}
}

// applyRunMethodDoc extracts and applies Run method documentation.
func applyRunMethodDoc(node *commandNode, typ reflect.Type) {
	ptrType := reflect.PointerTo(typ)

	runMethod, hasRun := ptrType.MethodByName("Run")
	if !hasRun {
		return
	}

	node.RunMethod = reflect.Value{} // Marker

	if doc := getMethodDoc(runMethod); doc != "" {
		node.Description = strings.TrimSpace(doc)
	}
}

func applyTagOptionsOverride(
	inst reflect.Value,
	field reflect.StructField,
	opts TagOptions,
) (TagOptions, error) {
	method := tagOptionsMethod(inst)
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

	return extractTagOptionsResult(results, opts)
}

func applyTagPart(opts *TagOptions, p string) {
	setters := []struct {
		prefix string
		apply  func(opts *TagOptions, val string)
	}{
		{"name=", func(opts *TagOptions, val string) { opts.Name = val }},
		{"subcommand=", func(opts *TagOptions, val string) { opts.Name = val }},
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
			return
		}
	}

	if p == "required" {
		opts.Required = true
	}
}

// applyTimeout extracts timeout from args and creates context with deadline.
func applyTimeout(
	ctx context.Context,
	args []string,
	opts RunOptions,
) (context.Context, []string, context.CancelFunc, error) {
	if opts.DisableTimeout {
		return ctx, args, nil, nil
	}

	timeout, remaining, err := extractTimeout(args)
	if err != nil {
		return ctx, args, nil, err
	}

	if timeout <= 0 {
		return ctx, remaining, nil, nil
	}

	newCtx, cancel := context.WithTimeout(ctx, timeout)

	return newCtx, remaining, cancel, nil
}

func assignSubcommandField(
	parent *commandNode,
	parentInst reflect.Value,
	subName string,
	sub *commandNode,
) error {
	if parent == nil || parent.Type == nil {
		return nil
	}

	typ := parent.Type
	for i := range typ.NumField() {
		field := typ.Field(i)

		opts, err := tagOptionsForField(parentInst, field)
		if err != nil {
			return err
		}

		if opts.Kind != TagKindSubcommand || opts.Name != subName {
			continue
		}

		return assignSubcommandValue(parentInst.Field(i), field.Type, sub, subName)
	}

	return nil
}

func assignSubcommandValue(
	fieldVal reflect.Value,
	fieldType reflect.Type,
	sub *commandNode,
	subName string,
) error {
	switch fieldType.Kind() { //nolint:exhaustive // default handles unsupported types
	case reflect.Func:
		if fieldVal.IsNil() {
			return fmt.Errorf("%w: %s", errSubcommandNil, subName)
		}

		sub.Func = fieldVal
	case reflect.Ptr:
		newInst := reflect.New(fieldType.Elem())
		fieldVal.Set(newInst)
		sub.Value = newInst.Elem()
	case reflect.Struct:
		newInst := reflect.New(fieldType).Elem()
		fieldVal.Set(newInst)
		sub.Value = newInst
	default:
		return fmt.Errorf("%w %s for %s", errUnsupportedFieldType, fieldType.Kind(), subName)
	}

	return nil
}

// --- Help output ---

func binaryName() string {
	if name := os.Getenv("TARG_BIN_NAME"); name != "" {
		return name
	}

	if len(os.Args) == 0 {
		return "targ"
	}

	name := os.Args[0]
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	if idx := strings.LastIndex(name, "\\"); idx != -1 {
		name = name[idx+1:]
	}

	return name
}

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
		name := positionalName(item)
		if item.Required {
			parts = append(parts, name)
		} else {
			parts = append(parts, fmt.Sprintf("[%s...]", name))
		}
	}

	return parts, nil
}

func buildUsageLine(node *commandNode) (string, error) {
	parts, err := buildUsageParts(node)
	if err != nil {
		return "", err
	}

	return strings.Join(parts, " "), nil
}

// buildUsageLineForPath builds a usage line with the given command path.
//
//nolint:unparam // binName is parameterized for testing and production flexibility
func buildUsageLineForPath(node *commandNode, binName, fullPath string) string {
	parts := buildUsagePartsForPath(node, binName, fullPath)
	return strings.Join(parts, " ")
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

// buildUsagePartsForPath builds usage parts with the given command path.
func buildUsagePartsForPath(node *commandNode, binName, fullPath string) []string {
	parts := []string{binName, fullPath}

	flags, err := collectFlagHelp(node)
	if err == nil {
		for _, item := range flags {
			flagStr := formatFlagUsageWrapped(item)
			if item.Required {
				parts = append(parts, flagStr)
			} else {
				parts = append(parts, "["+flagStr+"]")
			}
		}
	}

	if len(node.Subcommands) > 0 {
		parts = append(parts, "[subcommand]")
	}

	positionals, err := collectPositionalHelp(node)
	if err == nil {
		for _, item := range positionals {
			name := item.Name
			if item.Placeholder != "" {
				name = item.Placeholder
			}

			if name == "" {
				name = "ARG"
			}

			if item.Required {
				parts = append(parts, name)
			} else {
				parts = append(parts, "["+name+"]")
			}
		}
	}

	return parts
}

// callFunctionWithArgs calls a function with context and/or struct args.
// Handles: func(), func(ctx), func(args), func(ctx, args)
func callFunctionWithArgs(ctx context.Context, fn reflect.Value, argsInst reflect.Value) error {
	if !fn.IsValid() || (fn.Kind() == reflect.Func && fn.IsNil()) {
		return errNilFunctionCommand
	}

	ft := fn.Type()
	var callArgs []reflect.Value

	for i := range ft.NumIn() {
		paramType := ft.In(i)

		// Check if this param is context.Context
		if isContextType(paramType) {
			callArgs = append(callArgs, reflect.ValueOf(ctx))
			continue
		}

		// Otherwise it's the args struct - use the parsed instance
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

func callMethod(ctx context.Context, receiver reflect.Value, name string) error {
	method, ok := lookupMethod(receiver, name)
	if !ok {
		return nil
	}

	callArgs, err := validateMethodInputs(ctx, method, name)
	if err != nil {
		return err
	}

	err = validateMethodOutputs(method, name)
	if err != nil {
		return err
	}

	return invokeMethod(method, callArgs)
}

func callStringMethod(v reflect.Value, typ reflect.Type, method string) string {
	m := methodValue(v, typ, method)
	if !m.IsValid() {
		return ""
	}

	if m.Type().NumIn() != 0 || m.Type().NumOut() != 1 ||
		m.Type().Out(0).Kind() != reflect.String {
		return ""
	}

	out := m.Call(nil)
	if len(out) == 0 {
		return ""
	}

	s, ok := out[0].Interface().(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(s)
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

func collectFlagHelp(node *commandNode) ([]flagHelp, error) {
	if node.Type == nil {
		return nil, nil
	}

	typ := node.Type
	inst := tagOptionsInstance(node)

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

func collectPositionalHelp(node *commandNode) ([]positionalHelp, error) {
	if node.Type == nil {
		return nil, nil
	}

	typ := node.Type
	inst := tagOptionsInstance(node)

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

// completionExample returns a shell-specific completion setup example.
func completionExample() Example {
	shell := detectCurrentShell()

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

func copySubcommandFuncs(inst reflect.Value, node *commandNode) error {
	if !node.Value.IsValid() || node.Value.Kind() != reflect.Struct {
		return nil
	}

	typ := node.Type
	for i := range typ.NumField() {
		field := typ.Field(i)

		opts, err := tagOptionsForField(node.Value, field)
		if err != nil {
			return err
		}

		if opts.Kind == TagKindSubcommand && field.Type.Kind() == reflect.Func {
			inst.Field(i).Set(node.Value.Field(i))
		}
	}

	return nil
}

// createCommandNode creates a new command node with default values.
func createCommandNode(v reflect.Value, typ reflect.Type) *commandNode {
	return &commandNode{
		Name:        camelToKebab(typ.Name()),
		Type:        typ,
		Value:       v,
		Subcommands: make(map[string]*commandNode),
	}
}

// detectCurrentShell returns the name of the current shell.
func detectCurrentShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "unknown"
	}

	return filepath.Base(shell)
}

func detectTagKind(opts *TagOptions, tag, fieldName string) {
	if strings.Contains(tag, "subcommand") {
		opts.Kind = TagKindSubcommand
		opts.Name = camelToKebab(fieldName)
	}

	if strings.Contains(tag, "positional") {
		opts.Kind = TagKindPositional
		opts.Name = fieldName
	}
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
	inst, err := nodeInstance(node)
	if err != nil {
		return nil, err
	}

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

	err = runPersistentHooks(ctx, parents, "PersistentBefore")
	if err != nil {
		return nil, err
	}

	err = callFunctionWithArgs(ctx, node.Func, inst)
	if err != nil {
		return nil, err
	}

	err = runPersistentHooks(ctx, reverseChain(parents), "PersistentAfter")
	if err != nil {
		return nil, err
	}

	return result.remaining, nil
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

func extractTagOptionsResult(results []reflect.Value, fallback TagOptions) (TagOptions, error) {
	if len(results) < tagOptsReturnCount {
		return fallback, errTagOptsWrongValueCount
	}

	if !results[1].IsNil() {
		err, ok := results[1].Interface().(error)
		if !ok {
			return fallback, errTagOptsNonErrorType
		}

		return fallback, err
	}

	tagOpts, ok := results[0].Interface().(TagOptions)
	if !ok {
		return fallback, errTagOptsWrongType
	}

	return tagOpts, nil
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

func formatFlagName(item flagHelp) string {
	name := "--" + item.Name
	if item.Short != "" {
		name = fmt.Sprintf("--%s, -%s", item.Name, item.Short)
	}

	if item.Placeholder != "" && item.Placeholder != flagPlaceholder {
		name = fmt.Sprintf("%s %s", name, item.Placeholder)
	}

	return name
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

// formatFlagUsageWrapped formats a flag for use in usage line.
func formatFlagUsageWrapped(item flagHelp) string {
	name := "--" + item.Name
	if item.Short != "" {
		name = fmt.Sprintf("--%s, -%s", item.Name, item.Short)
	}

	if item.Placeholder != "" && item.Placeholder != "[flag]" {
		name = fmt.Sprintf("%s %s", name, item.Placeholder)
	}

	return name
}

func funcSourceFile(v reflect.Value) string {
	if !v.IsValid() || v.Kind() != reflect.Func || v.IsNil() {
		return ""
	}

	fn := runtime.FuncForPC(v.Pointer())
	if fn == nil {
		return ""
	}

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

// --- Command metadata helpers ---

func getCommandName(v reflect.Value, typ reflect.Type) string {
	name := callStringMethod(v, typ, "Name")
	if name == "" {
		return ""
	}

	return camelToKebab(name)
}

func getDescription(v reflect.Value, typ reflect.Type) string {
	desc := callStringMethod(v, typ, "Description")
	return strings.TrimSpace(desc)
}

// getNodeSourceFile returns the source file for a node, falling back to subcommands.

// getStructSourceFile returns the source file for a struct, checking for a SourceFile() method first.
func getStructSourceFile(v reflect.Value, typ reflect.Type) string {
	// Try calling SourceFile() method if it exists
	if file := callStringMethod(v, typ, "SourceFile"); file != "" {
		return file
	}

	// Fall back to runtime detection
	return structSourceFile(typ)
}

// handleHelpFlag checks for --help flag and prints help if requested.
func handleHelpFlag(n *commandNode, args []string, opts RunOptions) ([]string, bool) {
	if opts.DisableHelp || opts.HelpOnly {
		return nil, false
	}

	helpRequested, remaining := extractHelpFlag(args)
	if !helpRequested {
		return nil, false
	}

	printCommandHelp(n, opts)

	return remaining, true
}

func invokeMethod(method reflect.Value, args []reflect.Value) error {
	results := method.Call(args)
	if len(results) == 1 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return err
		}
	}

	return nil
}

func isContextType(t reflect.Type) bool {
	return t == reflect.TypeFor[context.Context]()
}

func isErrorType(t reflect.Type) bool {
	return t.Implements(reflect.TypeFor[error]())
}

func lookupMethod(receiver reflect.Value, name string) (reflect.Value, bool) {
	if !receiver.IsValid() {
		return reflect.Value{}, false
	}

	target := receiver
	if receiver.Kind() != reflect.Ptr && receiver.CanAddr() {
		target = receiver.Addr()
	}

	method := target.MethodByName(name)

	return method, method.IsValid()
}

func methodValue(v reflect.Value, typ reflect.Type, method string) reflect.Value {
	if v.IsValid() {
		if m := v.MethodByName(method); m.IsValid() {
			return m
		}

		if v.CanAddr() {
			if m := v.Addr().MethodByName(method); m.IsValid() {
				return m
			}
		}
	}

	ptr := reflect.New(typ)
	if m := ptr.MethodByName(method); m.IsValid() {
		return m
	}

	if m := ptr.Elem().MethodByName(method); m.IsValid() {
		return m
	}

	return reflect.Value{}
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

func nodeInstance(node *commandNode) (reflect.Value, error) {
	if nodeHasAddressableValue(node) {
		return node.Value, nil
	}

	if node == nil || node.Type == nil {
		return reflect.Value{}, nil
	}

	inst := reflect.New(node.Type).Elem()

	err := copySubcommandFuncs(inst, node)
	if err != nil {
		return reflect.Value{}, err
	}

	return inst, nil
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

func parseStruct(t any) (*commandNode, error) {
	v, typ, err := resolveStructValue(t)
	if err != nil {
		return nil, err
	}

	err = validateZeroFields(v, typ)
	if err != nil {
		return nil, err
	}

	node := createCommandNode(v, typ)
	applyRunMethodDoc(node, typ)
	applyNameAndDescription(node, v, typ)
	node.SourceFile = getStructSourceFile(v, typ)

	err = parseSubcommandFields(node, v, typ)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// parseSubcommandField parses a single subcommand field.
func parseSubcommandField(field reflect.StructField) (*commandNode, error) {
	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	if fieldType.Kind() == reflect.Func {
		err := validateFuncType(field.Type)
		if err != nil {
			return nil, err
		}

		return &commandNode{
			Func:        reflect.Zero(field.Type),
			Subcommands: make(map[string]*commandNode),
		}, nil
	}

	zeroVal := reflect.New(fieldType).Interface()

	return parseStruct(zeroVal)
}

// parseSubcommandFields parses all subcommand fields and adds them to the node.
func parseSubcommandFields(node *commandNode, v reflect.Value, typ reflect.Type) error {
	for i := range typ.NumField() {
		field := typ.Field(i)

		opts, err := tagOptionsForField(v, field)
		if err != nil {
			return err
		}

		if opts.Kind != TagKindSubcommand {
			continue
		}

		subNode, err := parseSubcommandField(field)
		if err != nil {
			return err
		}

		subNode.Parent = node
		subNode.Name = opts.Name
		node.Subcommands[subNode.Name] = subNode
	}

	return nil
}

func parseTagParts(opts *TagOptions, tag string) {
	parts := strings.SplitSeq(tag, ",")
	for p := range parts {
		applyTagPart(opts, strings.TrimSpace(p))
	}
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

	return parseStruct(t)
}

// parseTargetLike creates a commandNode from a TargetLike (targ.Target).
func parseTargetLike(target TargetLike) (*commandNode, error) {
	fn := target.Fn()
	if fn == nil {
		return nil, errNilTarget
	}

	// Handle string (shell command) vs function
	if cmd, ok := fn.(string); ok {
		return parseTargetLikeString(target, cmd)
	}

	return parseTargetLikeFunc(target, fn)
}

// parseTargetLikeFunc creates a commandNode for a function target.
func parseTargetLikeFunc(target TargetLike, fn any) (*commandNode, error) {
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		return nil, errTargetInvalidFnType
	}

	ft := fv.Type()

	// Validate return type
	if ft.NumOut() > 1 || (ft.NumOut() == 1 && !isErrorType(ft.Out(0))) {
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
		if isContextType(paramType) {
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
func parseTargetLikeString(target TargetLike, cmd string) (*commandNode, error) {
	node := &commandNode{
		Name:        target.GetName(),
		Description: target.GetDescription(),
		Subcommands: make(map[string]*commandNode),
	}

	// If no name set, use the first word of the command
	if node.Name == "" && cmd != "" {
		parts := strings.Fields(cmd)
		if len(parts) > 0 {
			node.Name = parts[0]
		}
	}

	return node, nil
}

// positionalName returns the display name for a positional argument.
func positionalName(item positionalHelp) string {
	if item.Placeholder != "" {
		return item.Placeholder
	}

	if item.Name != "" {
		return item.Name
	}

	return "ARG"
}

func printCommandHelp(node *commandNode, opts RunOptions) {
	binName := binaryName()

	// Description first (consistent with top-level)
	printDescription(node.Description)

	// Usage with targ flags and ... notation
	usageParts, err := buildUsageParts(node)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Insert "[targ flags...]" after binName
	usageParts = append([]string{binName, "[targ flags...]"}, usageParts...)
	printWrappedUsage("Usage: ", usageParts)
	fmt.Println()

	// Targ flags (not root, so no --completion)
	printTargFlags(opts, false)

	// Flags for this command
	flags, err := collectFlagHelp(node)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(flags) > 0 {
		fmt.Println("\nFlags:")
		printFlagsIndented(flags, "  ")
	}

	// Subcommands (list, not recursive details)
	if len(node.Subcommands) > 0 {
		fmt.Println("\nSubcommands:")
		printSubcommandList(node.Subcommands, "  ")
	}

	// Examples and More Info (at the very end)
	printExamples(opts, false)
	printMoreInfo(opts)
}

func printCommandSummary(node *commandNode, indent string) {
	fmt.Printf("%s%-20s %s\n", indent, node.Name, node.Description)

	// Recursively print subcommands
	// Sort subcommands by name for consistent output
	subcommandNames := make([]string, 0, len(node.Subcommands))
	for name := range node.Subcommands {
		subcommandNames = append(subcommandNames, name)
	}

	sort.Strings(subcommandNames)

	for _, name := range subcommandNames {
		sub := node.Subcommands[name]
		if sub != nil {
			printCommandSummary(sub, indent+"  ")
		}
	}
}

func printDescription(desc string) {
	if desc != "" {
		fmt.Println(desc)
		fmt.Println()
	}
}

func printExamples(opts RunOptions, isRoot bool) {
	examples := opts.Examples
	if examples == nil {
		if isRoot {
			examples = BuiltinExamples()
		} else {
			// At subcommand level, only show chaining example
			examples = []Example{BuiltinExamples()[1]}
		}
	}

	printExampleList(examples)
}

func printExamplesForNodes(opts RunOptions, nodes []*commandNode) {
	examples := opts.Examples
	if examples == nil {
		examples = builtinExamplesForNodes(nodes)
	}

	printExampleList(examples)
}

func printExampleList(examples []Example) {
	if len(examples) == 0 {
		return
	}

	fmt.Println("\nExamples:")

	for _, ex := range examples {
		fmt.Printf("  %s:\n", ex.Title)
		fmt.Printf("    %s\n", ex.Code)
	}
}

// printFlagWithWrappedEnum prints a flag, wrapping long enum values.
func printFlagWithWrappedEnum(name, usage, placeholder, indent string) {
	// Check if placeholder contains enum values that need wrapping
	if strings.HasPrefix(placeholder, "{") && strings.Contains(placeholder, "|") &&
		len(placeholder) > 40 {
		// For long enums, wrap the values across multiple lines
		enumContent := strings.TrimPrefix(strings.TrimSuffix(placeholder, "}"), "{")
		values := strings.Split(enumContent, "|")

		// Build the flag name without placeholder (we'll show enum separately)
		baseName := strings.TrimSuffix(name, " "+placeholder)

		// Print first line with flag name and first enum value
		fmt.Printf("%s%s {%s|\n", indent, baseName, values[0])

		// Print remaining values indented
		const bracePadding = 2 // account for " {" before enum values

		const usageSeparator = 4 // spaces between closing brace and usage text

		valueIndent := indent + strings.Repeat(" ", len(baseName)+bracePadding)
		for i := 1; i < len(values); i++ {
			if i == len(values)-1 {
				fmt.Printf(
					"%s%s}%s%s\n",
					valueIndent,
					values[i],
					strings.Repeat(" ", usageSeparator),
					usage,
				)
			} else {
				fmt.Printf("%s%s|\n", valueIndent, values[i])
			}
		}

		return
	}

	// Normal flag without long enum
	if usage != "" {
		fmt.Printf("%s%-28s %s\n", indent, name, usage)
	} else {
		fmt.Printf("%s%s\n", indent, name)
	}
}

// printFlagsIndented prints flags with proper indentation and enum wrapping.
func printFlagsIndented(flags []flagHelp, indent string) {
	for _, item := range flags {
		name := formatFlagName(item)
		printFlagWithWrappedEnum(name, item.Usage, item.Placeholder, indent)
	}
}

func printMoreInfo(opts RunOptions) {
	// User override takes precedence
	if opts.MoreInfoText != "" {
		fmt.Printf("\nMore info:\n  %s\n", opts.MoreInfoText)
		return
	}

	// Try explicit RepoURL, then auto-detect
	url := opts.RepoURL
	if url == "" {
		url = DetectRepoURL()
	}

	if url != "" {
		fmt.Printf("\nMore info:\n  %s\n", url)
	}
}

// printSubcommandGrid prints subcommands in a multi-column alphabetized grid.
func printSubcommandGrid(subs map[string]*commandNode, indent string) {
	names := sortedKeys(subs)
	if len(names) == 0 {
		return
	}

	// Calculate column width based on longest name
	maxLen := 0
	for _, name := range names {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	const padding = 2

	colWidth := maxLen + padding

	// Determine number of columns (fit in ~60 chars after indent)
	const availableWidth = 60

	numCols := max(availableWidth/colWidth, 1)

	// Print in columns
	fmt.Printf("%sSubcommands: ", indent)

	for i, name := range names {
		if i > 0 && i%numCols == 0 {
			fmt.Printf("\n%s             ", indent)
		}

		fmt.Printf("%-*s", colWidth, name)
	}

	fmt.Println()
}

// printSubcommandList prints subcommands with name and description only.
//
//nolint:unparam // indent kept for consistency with printSubcommandGrid
func printSubcommandList(subs map[string]*commandNode, indent string) {
	names := sortedKeys(subs)
	for _, name := range names {
		sub := subs[name]
		if sub == nil {
			continue
		}

		if sub.Description != "" {
			fmt.Printf("%s%-12s %s\n", indent, name, sub.Description)
		} else {
			fmt.Printf("%s%s\n", indent, name)
		}
	}
}

// printTargFlags prints targ's built-in flags.
func printTargFlags(opts RunOptions, isRoot bool) {
	fmt.Println("Targ flags:")

	if isRoot && !opts.DisableCompletion {
		fmt.Println("  --completion [shell]")
	}

	if !opts.DisableHelp {
		fmt.Println("  --help")
	}

	if !opts.DisableTimeout {
		fmt.Println("  --timeout <duration>")
	}
}

// printTopLevelCommand prints a top-level command with aligned description.
// width is the column width for the name (for alignment).
func printTopLevelCommand(node *commandNode, width int) {
	if node.Description != "" {
		fmt.Printf("  %-*s  %s\n", width, node.Name, node.Description)
	} else {
		fmt.Printf("  %s\n", node.Name)
	}
}

// maxNameWidth returns the maximum name length among nodes.
func maxNameWidth(nodes []*commandNode) int {
	max := 0
	for _, node := range nodes {
		if len(node.Name) > max {
			max = len(node.Name)
		}
	}
	return max
}

// getNodeSourceFile returns the source file for a node, checking subcommands if needed.
func getNodeSourceFile(node *commandNode) string {
	if node.SourceFile != "" {
		return node.SourceFile
	}
	for _, sub := range node.Subcommands {
		if sub.SourceFile != "" {
			return sub.SourceFile
		}
	}
	return ""
}

// groupNodesBySource groups nodes by their source file, preserving order.
func groupNodesBySource(nodes []*commandNode) []struct {
	source string
	nodes  []*commandNode
} {
	// Use a slice to preserve order, map for lookup
	var groups []struct {
		source string
		nodes  []*commandNode
	}
	sourceIndex := make(map[string]int)

	for _, node := range nodes {
		source := relativeSourcePath(getNodeSourceFile(node))
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

func printUsage(nodes []*commandNode, opts RunOptions) {
	binName := binaryName()

	// Description first (consistent with command-level)
	if opts.Description != "" {
		fmt.Println(opts.Description)
		fmt.Println()
	}

	fmt.Printf("Usage: %s [targ flags...] [<command>...]\n\n", binName)

	// Targ flags
	printTargFlags(opts, true)
	printValuesAndFormats(opts, true)

	// Commands grouped by source
	fmt.Println("\nCommands:")

	groups := groupNodesBySource(nodes)
	for i, group := range groups {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("\n  [%s]\n", group.source)
		width := maxNameWidth(group.nodes)
		for _, node := range group.nodes {
			printTopLevelCommand(node, width)
		}
	}

	printExamplesForNodes(opts, nodes)
	printMoreInfo(opts)
}

// printValuesAndFormats prints the Values and Formats help sections.
func printValuesAndFormats(opts RunOptions, isRoot bool) {
	// Values section (only if completion is shown)
	if isRoot && !opts.DisableCompletion {
		shell := detectCurrentShell()

		fmt.Println("\nValues:")
		fmt.Printf("  shell: bash, zsh, fish (default: current shell (detected: %s))\n", shell)
	}

	// Formats section
	if !opts.DisableTimeout || isRoot {
		fmt.Println("\nFormats:")

		if !opts.DisableTimeout {
			fmt.Println("  duration: <int><unit> where unit is s (seconds), m (minutes), h (hours)")
		}
	}
}

// printWrappedUsage prints a usage line with wrapping at word boundaries.
// prefix is the text before the usage line (e.g., "Usage: " or "  Usage: ")
// parts are the individual components (flags, positionals, etc.)
func printWrappedUsage(prefix string, parts []string) {
	writeWrappedUsage(os.Stdout, prefix, parts)
}

// relativeSourcePath returns a relative path if possible, otherwise the absolute path.
func relativeSourcePath(absPath string) string {
	return relativeSourcePathWithGetwd(absPath, os.Getwd)
}

// relativeSourcePathWithGetwd is a testable version that accepts a working directory getter.
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

	// If the relative path starts with "..", it might be cleaner to show absolute
	// But for now, always prefer relative
	return relPath
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

// resolveStructValue extracts the reflect.Value and Type from a target.
func resolveStructValue(t any) (reflect.Value, reflect.Type, error) {
	if t == nil {
		return reflect.Value{}, nil, errNilTarget
	}

	v := reflect.ValueOf(t)
	typ := v.Type()

	if typ.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}, nil, errNilPointerTarget
		}

		typ = typ.Elem()
		v = v.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return reflect.Value{}, nil, fmt.Errorf("%w, got %v", errExpectedStruct, typ.Kind())
	}

	return v, typ, nil
}

func reverseChain(chain []commandInstance) []commandInstance {
	if len(chain) == 0 {
		return nil
	}

	out := make([]commandInstance, len(chain))
	for i := range chain {
		out[i] = chain[len(chain)-1-i]
	}

	return out
}

func runCommand(
	ctx context.Context,
	node *commandNode,
	inst reflect.Value,
	_ []string,
	_ int,
) error {
	if node == nil {
		return nil
	}

	err := callMethod(ctx, inst, "Run")

	return err
}

func runPersistentHooks(ctx context.Context, chain []commandInstance, methodName string) error {
	for _, inst := range chain {
		if inst.node == nil || inst.node.Type == nil {
			continue
		}

		err := callMethod(ctx, inst.value, methodName)
		if err != nil {
			return err
		}
	}

	return nil
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

// structSourceFile returns the source file for a struct's Run method.
func structSourceFile(typ reflect.Type) string {
	ptrType := reflect.PointerTo(typ)

	runMethod, hasRun := ptrType.MethodByName("Run")
	if !hasRun {
		return ""
	}

	pc := runMethod.Func.Pointer()

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}

	file, _ := fn.FileLine(pc)

	return file
}

// --- Tag options ---

func tagOptionsForField(inst reflect.Value, field reflect.StructField) (TagOptions, error) {
	tag := field.Tag.Get("targ")

	opts := TagOptions{
		Kind: TagKindFlag,
		Name: strings.ToLower(field.Name),
	}
	if strings.TrimSpace(tag) == "" {
		overridden, err := applyTagOptionsOverride(inst, field, opts)
		if err != nil {
			return TagOptions{}, err
		}

		return overridden, nil
	}

	detectTagKind(&opts, tag, field.Name)
	parseTagParts(&opts, tag)

	overridden, err := applyTagOptionsOverride(inst, field, opts)
	if err != nil {
		return TagOptions{}, err
	}

	return overridden, nil
}

func tagOptionsInstance(node *commandNode) reflect.Value {
	if node == nil {
		return reflect.Value{}
	}

	if node.Value.IsValid() {
		return node.Value
	}

	if node.Type != nil {
		return reflect.New(node.Type).Elem()
	}

	return reflect.Value{}
}

func tagOptionsMethod(inst reflect.Value) reflect.Value {
	if !inst.IsValid() {
		return reflect.Value{}
	}

	target := inst
	if inst.Kind() != reflect.Ptr {
		if inst.CanAddr() {
			target = inst.Addr()
		}
	}

	return target.MethodByName("TagOptions")
}

// validateFieldValue checks that a field is properly zero-valued.
func validateFieldValue(fieldVal reflect.Value, opts TagOptions, typeName, fieldName string) error {
	if opts.Kind == TagKindSubcommand {
		if fieldVal.Kind() == reflect.Func || fieldVal.IsZero() {
			return nil
		}

		return fmt.Errorf("command %s %w: %s", typeName, errSubcommandPrefilled, fieldName)
	}

	if !fieldVal.IsZero() {
		return fmt.Errorf("command %s %w", typeName, errFieldNotZero)
	}

	return nil
}

func validateFuncType(typ reflect.Type) error {
	if typ.NumIn() > 1 {
		return errFuncTooManyInputs
	}

	if typ.NumIn() == 1 && !isContextType(typ.In(0)) {
		return errFuncMustAcceptContext
	}

	if typ.NumOut() == 0 {
		return nil
	}

	if typ.NumOut() == 1 && isErrorType(typ.Out(0)) {
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

func validateMethodInputs(
	ctx context.Context,
	method reflect.Value,
	name string,
) ([]reflect.Value, error) {
	mtype := method.Type()
	if mtype.NumIn() > 1 {
		return nil, fmt.Errorf("%s %w", name, errMethodTooManyInputs)
	}

	if mtype.NumIn() == 0 {
		return nil, nil
	}

	if !isContextType(mtype.In(0)) {
		return nil, fmt.Errorf("%s %w", name, errMethodMustAcceptContext)
	}

	return []reflect.Value{reflect.ValueOf(ctx)}, nil
}

func validateMethodOutputs(method reflect.Value, name string) error {
	mtype := method.Type()
	if mtype.NumOut() > 1 {
		return fmt.Errorf("%s %w", name, errMethodMustReturnError)
	}

	if mtype.NumOut() == 1 && !isErrorType(mtype.Out(0)) {
		return fmt.Errorf("%s %w", name, errMethodMustReturnError)
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

	if mtype.Out(0) != reflect.TypeFor[TagOptions]() || !isErrorType(mtype.Out(1)) {
		return errTagOptsInvalidOutput
	}

	return nil
}

// validateZeroFields ensures fields are zero-valued (defaults should use tags).
func validateZeroFields(v reflect.Value, typ reflect.Type) error {
	for i := range typ.NumField() {
		field := typ.Field(i)

		opts, err := tagOptionsForField(v, field)
		if err != nil {
			return err
		}

		err = validateFieldValue(v.Field(i), opts, typ.Name(), field.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

// writeWrappedUsage writes a usage line with wrapping at word boundaries.
func writeWrappedUsage(w io.Writer, prefix string, parts []string) {
	if len(parts) == 0 {
		_, _ = fmt.Fprintln(w, prefix)
		return
	}

	// Build lines by adding parts until we exceed the target width
	currentLine := prefix + parts[0]
	indent := strings.Repeat(" ", len(prefix))

	for i := 1; i < len(parts); i++ {
		part := parts[i] //nolint:gosec // bounds checked by loop condition and empty check above
		if len(currentLine)+1+len(part) <= usageLineWidth {
			currentLine += " " + part
		} else {
			_, _ = fmt.Fprintln(w, currentLine)
			currentLine = indent + part
		}
	}

	_, _ = fmt.Fprintln(w, currentLine)
}
