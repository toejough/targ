package core

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/toejough/targ/internal/flags"
)

// PrintCompletionScriptTo writes a shell completion script to the given writer.
func PrintCompletionScriptTo(w io.Writer, shell, binName string) error {
	var err error

	switch shell {
	case "bash":
		_, err = fmt.Fprintf(w, _bashCompletion, binName, binName, binName, binName)
	case zshShell:
		_, err = fmt.Fprintf(w, _zshCompletion, binName, binName, binName, binName, binName)
	case fishShell:
		_, err = fmt.Fprintf(w, _fishCompletion, binName, binName, binName, binName)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedShell, shell)
	}

	if err != nil {
		return fmt.Errorf("writing completion script: %w", err)
	}

	return nil
}

// unexported constants.
const (
	_bashCompletion = `
_%s_completion() {
    local request="${COMP_LINE}"
    local completions
    completions=$(%s __complete "$request")

    COMPREPLY=( $(compgen -W "$completions" -- "${COMP_WORDS[COMP_CWORD]}") )
}
complete -F _%s_completion %s
`
	_fishCompletion = `
function __%s_complete
    set -l request (commandline -cp)
    %s __complete "$request"
end
complete -c %s -a "(__%s_complete)" -f
`
	_zshCompletion = `
#compdef %s

_%s_completion() {
    local request="${words[*]}"
    local completions
    completions=("${(@f)$(%s __complete "$request")}")

    compadd -a completions
}
compdef _%s_completion %s
`
	bashShell          = "bash"
	fishShell          = "fish"
	singleShortFlagLen = 2
	zshShell           = "zsh"
)

// unexported variables.
var (
	errUnsupportedShell = errors.New("unsupported shell")
)

type cmdLineTokenizer struct {
	parts    []string
	current  strings.Builder
	inSingle bool
	inDouble bool
	escaped  bool
	isNewArg bool
}

func (t *cmdLineTokenizer) finalize() {
	if t.escaped {
		t.current.WriteByte('\\')
	}

	t.flushCurrent()

	if t.inSingle || t.inDouble {
		t.isNewArg = false
	}
}

func (t *cmdLineTokenizer) flushCurrent() {
	if t.current.Len() > 0 {
		t.parts = append(t.parts, t.current.String())
		t.current.Reset()
	}
}

func (t *cmdLineTokenizer) handleSpecialChar(ch byte) bool {
	if ch == '\\' && !t.inSingle {
		t.escaped = true
		t.isNewArg = false

		return true
	}

	if ch == '\'' && !t.inDouble {
		t.inSingle = !t.inSingle
		t.isNewArg = false

		return true
	}

	if ch == '"' && !t.inSingle {
		t.inDouble = !t.inDouble
		t.isNewArg = false

		return true
	}

	if isWhitespace(ch) && !t.inSingle && !t.inDouble {
		t.flushCurrent()
		t.isNewArg = true

		return true
	}

	return false
}

func (t *cmdLineTokenizer) processChar(ch byte) {
	if t.escaped {
		t.current.WriteByte(ch)
		t.escaped = false
		t.isNewArg = false

		return
	}

	if t.handleSpecialChar(ch) {
		return
	}

	t.current.WriteByte(ch)
	t.isNewArg = false
}

func (t *cmdLineTokenizer) tokenize(commandLine string) {
	for i := range len(commandLine) {
		t.processChar(commandLine[i])
	}

	t.finalize()
}

type completionFlagSpec struct {
	TakesValue bool
	Variadic   bool
}

type completionState struct {
	w                   io.Writer
	roots               []*commandNode
	prefix              string
	processedArgs       []string
	isNewArg            bool
	currentNode         *commandNode
	chain               []commandInstance
	singleRoot          bool
	atRoot              bool
	allowRootSuggests   bool
	positionalsComplete bool
	explicit            bool
}

// findRootByName finds a root command by name (case-insensitive).
func (s *completionState) findRootByName(name string) *commandNode {
	for _, r := range s.roots {
		if strings.EqualFold(r.Name, name) {
			return r
		}
	}

	return nil
}

// followRemaining handles remaining args after parsing in multi-root mode.
// In single-root mode, remaining args would cause a parse error, not reach this path.
func (s *completionState) followRemaining(result parseResult) bool {
	nextRoot := findCompletionRoot(s.roots, result.remaining[0])
	if nextRoot == nil {
		return false
	}

	s.currentNode = nextRoot
	s.processedArgs = result.remaining[1:]
	s.explicit = true
	s.atRoot = false

	return true
}

// followSubcommand updates state to follow a subcommand.
func (s *completionState) followSubcommand(result parseResult) {
	s.currentNode = result.subcommand
	s.processedArgs = result.remaining
	s.explicit = true
	s.atRoot = false
}

// resolveCommandChain follows subcommands to find the current context.
func (s *completionState) resolveCommandChain() {
	for {
		nextChain, result, err := completionParse(s.currentNode, s.processedArgs, s.explicit)
		if err != nil {
			// For completion, we still want the chain even if parsing partially failed.
			// This handles cases like grouped short flags with a value-taking flag at the end (-vao).
			if errors.Is(err, errShortFlagGroupNotBool) && len(nextChain) > 0 {
				s.chain = nextChain
			}

			return
		}

		s.chain = nextChain
		s.positionalsComplete = result.positionalsComplete

		if result.subcommand != nil {
			s.followSubcommand(result)
			continue
		}

		if len(result.remaining) > 0 {
			if s.followRemaining(result) {
				continue
			}

			return
		}

		break
	}
}

// resolveInitialRoot sets up the initial command node.
func (s *completionState) resolveInitialRoot() bool {
	if s.singleRoot {
		s.currentNode = s.roots[0]
		s.explicit = false

		return false
	}

	if len(s.processedArgs) == 0 {
		s.suggestRootsAndFlags()
		return true
	}

	s.currentNode = s.findRootByName(s.processedArgs[0])
	if s.currentNode == nil {
		s.suggestMatchingRoots(s.processedArgs[0])
		return true
	}

	s.processedArgs = s.processedArgs[1:]
	s.atRoot = false
	s.explicit = true

	return false
}

// suggestCommands suggests subcommands, siblings, and special tokens.
func (s *completionState) suggestCommands() {
	if s.singleRoot && s.atRoot {
		printIfPrefix(s.w, s.currentNode.Name, s.prefix)
	}

	for name := range s.currentNode.Subcommands {
		printIfPrefix(s.w, name, s.prefix)
	}

	if s.currentNode.Parent != nil {
		for name := range s.currentNode.Parent.Subcommands {
			printIfPrefix(s.w, name, s.prefix)
		}
	}

	if !s.atRoot {
		printIfPrefix(s.w, "^", s.prefix)
	}
}

// suggestCompletions outputs completion suggestions.
func (s *completionState) suggestCompletions() error {
	s.suggestCommands()

	done, err := s.suggestEnumValues()
	if err != nil || done {
		return err
	}

	err = s.suggestFlagsIfNeeded()
	if err != nil {
		return err
	}

	if strings.HasPrefix(s.prefix, "-") {
		return nil
	}

	return s.suggestPositionalEnumsOrRoots()
}

// suggestEnumValues suggests enum values if expecting a flag value.
func (s *completionState) suggestEnumValues() (bool, error) {
	values, ok, err := enumValuesForArg(s.chain, s.processedArgs, s.prefix, s.isNewArg)
	if err != nil {
		return false, err
	}

	if !ok {
		return false, nil
	}

	for _, value := range values {
		printIfPrefix(s.w, value, s.prefix)
	}

	return true, nil
}

// suggestFlagsIfNeeded suggests flags if prefix starts with - or is empty.
func (s *completionState) suggestFlagsIfNeeded() error {
	if !strings.HasPrefix(s.prefix, "-") && s.prefix != "" {
		return nil
	}

	return suggestFlags(s.w, s.chain, s.prefix, s.atRoot)
}

// suggestMatchingRoots suggests roots that match a partial prefix.
func (s *completionState) suggestMatchingRoots(partial string) {
	for _, r := range s.roots {
		printIfPrefix(s.w, r.Name, partial)
	}
}

// suggestPositionalEnums suggests enum values for the current positional.
func (s *completionState) suggestPositionalEnums() (bool, error) {
	posIndex, err := positionalIndex(s.currentNode, s.processedArgs, s.chain)
	if err != nil {
		return false, err
	}

	if len(s.chain) == 0 {
		return false, nil
	}

	fields, err := positionalFields(s.chain[len(s.chain)-1].node, s.chain[len(s.chain)-1].value)
	if err != nil {
		return false, err
	}

	if posIndex >= len(fields) || fields[posIndex].Opts.Enum == "" {
		return false, nil
	}

	for value := range strings.SplitSeq(fields[posIndex].Opts.Enum, "|") {
		printIfPrefix(s.w, value, s.prefix)
	}

	return true, nil
}

// suggestPositionalEnumsOrRoots suggests positional enums or root commands.
func (s *completionState) suggestPositionalEnumsOrRoots() error {
	specs, err := completionFlagSpecs(s.chain)
	if err != nil {
		return err
	}

	if expectingFlagValue(s.processedArgs, specs) {
		return nil
	}

	suggested, err := s.suggestPositionalEnums()
	if err != nil || suggested {
		return err
	}

	s.suggestRootsIfAllowed()

	return nil
}

// suggestRootsAndFlags suggests all roots and targ flags at root level.
func (s *completionState) suggestRootsAndFlags() {
	for _, r := range s.roots {
		printIfPrefix(s.w, r.Name, s.prefix)
	}

	for _, opt := range targGlobalFlags() {
		printIfPrefix(s.w, opt, s.prefix)
	}

	for _, opt := range targRootOnlyFlags() {
		printIfPrefix(s.w, opt, s.prefix)
	}
}

// suggestRootsIfAllowed suggests root commands if conditions are met.
func (s *completionState) suggestRootsIfAllowed() {
	if !s.allowRootSuggests || !s.positionalsComplete || strings.HasPrefix(s.prefix, "-") {
		return
	}

	for _, root := range s.roots {
		printIfPrefix(s.w, root.Name, s.prefix)
	}
}

type positionalCounter struct {
	args  []string
	specs map[string]completionFlagSpec
	index int
	count int
}

// countPositionals iterates through args and counts positional arguments.
func (c *positionalCounter) countPositionals() {
	for c.index < len(c.args) {
		c.processArg()

		c.index++
	}
}

// processArg handles a single argument at the current index.
func (c *positionalCounter) processArg() {
	arg := c.args[c.index]

	if arg == "--" {
		return
	}

	if strings.HasPrefix(arg, "--") {
		c.skipLongFlag(arg)
		return
	}

	if strings.HasPrefix(arg, "-") && len(arg) > 1 {
		c.skipShortFlag(arg)
		return
	}

	c.count++
}

// skipFlagValues skips value arguments for a flag.
func (c *positionalCounter) skipFlagValues(variadic bool) {
	if variadic {
		c.skipVariadicValues()
	} else if c.index+1 < len(c.args) {
		c.index++
	}
}

// skipGroupedShortFlags handles grouped short flags like -abc.
func (c *positionalCounter) skipGroupedShortFlags(arg string) {
	group := strings.TrimPrefix(arg, "-")

	for idx, ch := range group {
		spec, ok := c.specs["-"+string(ch)]
		if !ok || !spec.TakesValue {
			continue
		}

		if idx == len(group)-1 && c.index+1 < len(c.args) {
			c.index++
		}

		return
	}
}

// skipLongFlag handles --flag style arguments.
func (c *positionalCounter) skipLongFlag(arg string) {
	if strings.Contains(arg, "=") {
		return
	}

	spec, ok := c.specs[arg]
	if !ok || !spec.TakesValue {
		return
	}

	c.skipFlagValues(spec.Variadic)
}

// skipShortFlag handles -f and -abc style arguments.
func (c *positionalCounter) skipShortFlag(arg string) {
	if strings.Contains(arg, "=") {
		return
	}

	if len(arg) == singleShortFlagLen {
		c.skipSingleShortFlag(arg)
		return
	}

	c.skipGroupedShortFlags(arg)
}

// skipSingleShortFlag handles single short flags like -f.
func (c *positionalCounter) skipSingleShortFlag(arg string) {
	spec, ok := c.specs[arg]
	if !ok || !spec.TakesValue {
		return
	}

	c.skipFlagValues(spec.Variadic)
}

// skipVariadicValues skips all values for a variadic flag.
func (c *positionalCounter) skipVariadicValues() {
	for c.index+1 < len(c.args) {
		next := c.args[c.index+1]
		if next == "--" || strings.HasPrefix(next, "-") {
			break
		}

		c.index++
	}
}

type positionalField struct {
	Field reflect.StructField
	Opts  TagOptions
}

// addEnumIfNew adds enum values to the map if the key doesn't already exist.
func addEnumIfNew(enumByFlag map[string][]string, key string, values []string) {
	if _, exists := enumByFlag[key]; !exists {
		enumByFlag[key] = values
	}
}

// collectEnumsByFlag builds a map of flag names to their enum values.
func collectEnumsByFlag(chain []commandInstance) (map[string][]string, error) {
	enumByFlag := map[string][]string{}

	for _, current := range chain {
		err := collectInstanceEnums(current, enumByFlag)
		if err != nil {
			return nil, err
		}
	}

	return enumByFlag, nil
}

// collectFieldEnums collects enum values for a single field's flags.
func collectFieldEnums(
	current commandInstance,
	field reflect.StructField,
	enumByFlag map[string][]string,
) error {
	opts, err := tagOptionsForField(current.value, field)
	if err != nil {
		return err
	}

	if opts.Kind != TagKindFlag || opts.Enum == "" {
		return nil
	}

	enumValues := strings.Split(opts.Enum, "|")
	addEnumIfNew(enumByFlag, "--"+opts.Name, enumValues)

	if opts.Short != "" {
		addEnumIfNew(enumByFlag, "-"+opts.Short, enumValues)
	}

	return nil
}

// collectInstanceEnums collects enum values from a single command instance.
func collectInstanceEnums(current commandInstance, enumByFlag map[string][]string) error {
	if current.node == nil || current.node.Type == nil {
		return nil
	}

	for i := range current.node.Type.NumField() {
		err := collectFieldEnums(current, current.node.Type.Field(i), enumByFlag)
		if err != nil {
			return err
		}
	}

	return nil
}

func completionFlagSpecs(chain []commandInstance) (map[string]completionFlagSpec, error) {
	specs := map[string]completionFlagSpec{}

	for _, current := range chain {
		if current.node == nil || current.node.Type == nil {
			continue
		}

		inst := current.value

		for i := range current.node.Type.NumField() {
			field := current.node.Type.Field(i)

			opts, err := tagOptionsForField(inst, field)
			if err != nil {
				return nil, err
			}

			if opts.Kind != TagKindFlag {
				continue
			}

			takesValue := field.Type.Kind() != reflect.Bool
			variadic := field.Type.Kind() == reflect.Slice

			specs["--"+opts.Name] = completionFlagSpec{TakesValue: takesValue, Variadic: variadic}
			if opts.Short != "" {
				specs["-"+opts.Short] = completionFlagSpec{
					TakesValue: takesValue,
					Variadic:   variadic,
				}
			}
		}
	}

	return specs, nil
}

func completionParse(
	node *commandNode,
	args []string,
	explicit bool,
) ([]commandInstance, parseResult, error) {
	chainNodes := nodeChain(node)

	chain := make([]commandInstance, 0, len(chainNodes))

	for _, current := range chainNodes {
		inst := nodeInstance(current)
		chain = append(chain, commandInstance{node: current, value: inst})
	}

	if len(chain) == 0 {
		return nil, parseResult{}, nil
	}

	result, err := parseCommandArgs(
		node,
		chain[len(chain)-1].value,
		chain,
		args,
		map[string]bool{},
		explicit,
		false,
		true,
	)

	return chain, result, err
}

func doCompletion(w io.Writer, roots []*commandNode, commandLine string) error {
	state, done := prepareCompletionState(w, roots, commandLine)
	if done || state == nil {
		return nil
	}

	if done := state.resolveInitialRoot(); done {
		return nil
	}

	state.resolveCommandChain()

	return state.suggestCompletions()
}

func enumValuesForArg(
	chain []commandInstance,
	args []string,
	prefix string,
	isNewArg bool,
) ([]string, bool, error) {
	enumByFlag, err := collectEnumsByFlag(chain)
	if err != nil {
		return nil, false, err
	}

	if len(enumByFlag) == 0 || len(args) == 0 {
		return nil, false, nil
	}

	if !isNewArg && strings.HasPrefix(prefix, "-") {
		return nil, false, nil
	}

	previous := args[len(args)-1]
	if values, ok := enumByFlag[previous]; ok {
		return values, true, nil
	}

	// Handle grouped short flags like -vao where -o takes a value
	if isGroupedShortFlag(previous) {
		lastFlag := extractLastShortFlag(previous)
		if values, ok := enumByFlag[lastFlag]; ok {
			return values, true, nil
		}
	}

	return nil, false, nil
}

func expectingFlagValue(args []string, specs map[string]completionFlagSpec) bool {
	if len(args) == 0 {
		return false
	}

	last := args[len(args)-1]
	if last == "--" {
		return false
	}

	if strings.HasPrefix(last, "--") {
		return expectingLongFlagValue(last, specs)
	}

	if strings.HasPrefix(last, "-") {
		return expectingShortFlagValue(last, specs)
	}

	return false
}

func expectingGroupedShortFlagValue(flag string, specs map[string]completionFlagSpec) bool {
	group := strings.TrimPrefix(flag, "-")
	for i, ch := range group {
		spec, ok := specs["-"+string(ch)]
		if !ok {
			continue
		}

		if spec.TakesValue {
			return i == len(group)-1
		}
	}

	return false
}

func expectingLongFlagValue(flag string, specs map[string]completionFlagSpec) bool {
	if strings.Contains(flag, "=") {
		return false
	}

	spec, ok := specs[flag]

	return ok && spec.TakesValue
}

func expectingShortFlagValue(flag string, specs map[string]completionFlagSpec) bool {
	if len(flag) == singleShortFlagLen {
		spec, ok := specs[flag]

		return ok && spec.TakesValue
	}

	return expectingGroupedShortFlagValue(flag, specs)
}

// extractLastShortFlag extracts the last short flag from a grouped flag like -abc -> -c.
func extractLastShortFlag(arg string) string {
	group := strings.TrimPrefix(arg, "-")
	if len(group) == 0 {
		return arg
	}
	// Get the last rune as a string
	runes := []rune(group)

	return "-" + string(runes[len(runes)-1])
}

// extractPrefixAndArgs separates the current prefix from processed args.
func extractPrefixAndArgs(parts []string, isNewArg bool) (string, []string) {
	if !isNewArg && len(parts) > 0 {
		return parts[len(parts)-1], parts[:len(parts)-1]
	}

	return "", parts
}

func findCompletionRoot(roots []*commandNode, name string) *commandNode {
	for _, root := range roots {
		if strings.EqualFold(root.Name, name) {
			return root
		}
	}

	return nil
}

func hasFlagValuePrefix(arg string, flags map[string]bool) bool {
	for flag := range flags {
		if strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}

	return false
}

// isGroupedShortFlag checks if arg is a grouped short flag like -abc (not --long or -x).
func isGroupedShortFlag(arg string) bool {
	return strings.HasPrefix(arg, "-") &&
		!strings.HasPrefix(arg, "--") &&
		len(arg) > singleShortFlagLen &&
		!strings.Contains(arg, "=")
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

func positionalFields(node *commandNode, inst reflect.Value) ([]positionalField, error) {
	if node == nil || node.Type == nil {
		return nil, nil
	}

	fields := make([]positionalField, 0, node.Type.NumField())

	for i := range node.Type.NumField() {
		field := node.Type.Field(i)

		opts, err := tagOptionsForField(inst, field)
		if err != nil {
			return nil, err
		}

		if opts.Kind != TagKindPositional {
			continue
		}

		fields = append(fields, positionalField{Field: field, Opts: opts})
	}

	return fields, nil
}

func positionalIndex(_ *commandNode, args []string, chain []commandInstance) (int, error) {
	specs, err := completionFlagSpecs(chain)
	if err != nil {
		return 0, err
	}

	counter := &positionalCounter{args: args, specs: specs}
	counter.countPositionals()

	return counter.count, nil
}

// prepareCompletionState initializes completion state from command line.
func prepareCompletionState(
	w io.Writer,
	roots []*commandNode,
	commandLine string,
) (*completionState, bool) {
	parts, isNewArg := tokenizeCommandLine(commandLine)
	if len(parts) == 0 {
		return nil, true
	}

	parts = parts[1:] // Remove binary name

	prefix, processedArgs := extractPrefixAndArgs(parts, isNewArg)
	processedArgs = skipTargFlags(processedArgs)

	return &completionState{
		w:                 w,
		roots:             roots,
		prefix:            prefix,
		processedArgs:     processedArgs,
		isNewArg:          isNewArg,
		singleRoot:        len(roots) == 1,
		atRoot:            true,
		allowRootSuggests: len(roots) > 1,
	}, false
}

// printIfPrefix prints name if it has the given prefix.
func printIfPrefix(w io.Writer, name, prefix string) {
	if strings.HasPrefix(name, prefix) {
		_, _ = fmt.Fprintln(w, name)
	}
}

// skipTargFlags removes targ-level flags from the args for completion purposes.
// These flags are handled by the outer targ binary, not the bootstrap.
func skipTargFlags(args []string) []string {
	result := make([]string, 0, len(args))

	flagsWithValues := targFlagsWithValues()
	booleanFlags := targBooleanFlags()

	skip := false
	for _, arg := range args {
		if skip {
			skip = false
			continue
		}
		// Flags that take a value - skip flag and next arg
		if flagsWithValues[arg] {
			skip = true
			continue
		}
		// Flags with --flag=value syntax
		if hasFlagValuePrefix(arg, flagsWithValues) {
			continue
		}
		// Boolean flags
		if booleanFlags[arg] || hasFlagValuePrefix(arg, booleanFlags) {
			continue
		}

		result = append(result, arg)
	}

	return result
}

// suggestCommandFlags suggests flags from command chain fields.
func suggestCommandFlags(
	w io.Writer,
	chain []commandInstance,
	prefix string,
	seen map[string]bool,
) error {
	for _, current := range chain {
		err := suggestInstanceFlags(w, current, prefix, seen)
		if err != nil {
			return err
		}
	}

	return nil
}

// suggestFieldFlags suggests the long and short flags for a field.
func suggestFieldFlags(
	w io.Writer,
	current commandInstance,
	field reflect.StructField,
	prefix string,
	seen map[string]bool,
) error {
	opts, err := tagOptionsForField(current.value, field)
	if err != nil {
		return err
	}

	if opts.Kind != TagKindFlag {
		return nil
	}

	suggestFlag(w, "--"+opts.Name, prefix, seen)

	if opts.Short != "" {
		suggestFlag(w, "-"+opts.Short, prefix, seen)
	}

	return nil
}

// suggestFlag prints a single flag if it matches prefix and hasn't been seen.
func suggestFlag(w io.Writer, flag, prefix string, seen map[string]bool) {
	if strings.HasPrefix(flag, prefix) && !seen[flag] {
		_, _ = fmt.Fprintln(w, flag)
		seen[flag] = true
	}
}

func suggestFlags(w io.Writer, chain []commandInstance, prefix string, atRoot bool) error {
	if len(chain) == 0 {
		return nil
	}

	seen := map[string]bool{}

	err := suggestCommandFlags(w, chain, prefix, seen)
	if err != nil {
		return err
	}

	suggestMatchingFlags(w, targGlobalFlags(), prefix, seen)

	if atRoot {
		suggestMatchingFlags(w, targRootOnlyFlags(), prefix, seen)
	}

	return nil
}

// suggestInstanceFlags suggests flags from a single command instance.
func suggestInstanceFlags(
	w io.Writer,
	current commandInstance,
	prefix string,
	seen map[string]bool,
) error {
	if current.node == nil || current.node.Type == nil {
		return nil
	}

	typ := current.node.Type
	for i := range typ.NumField() {
		err := suggestFieldFlags(w, current, typ.Field(i), prefix, seen)
		if err != nil {
			return err
		}
	}

	return nil
}

// suggestMatchingFlags prints flags that match the prefix.
func suggestMatchingFlags(w io.Writer, flags []string, prefix string, seen map[string]bool) {
	for _, flag := range flags {
		suggestFlag(w, flag, prefix, seen)
	}
}

// targBooleanFlags returns flags that don't take a value.
func targBooleanFlags() map[string]bool { return flags.BooleanFlags() }

// targFlagsWithValues returns flags that consume the next argument as a value.
func targFlagsWithValues() map[string]bool { return flags.WithValues() }

// targGlobalFlags returns flags valid at any command level.
func targGlobalFlags() []string { return flags.GlobalFlags() }

// targRootOnlyFlags returns flags only valid at root level (before any command).
func targRootOnlyFlags() []string { return flags.RootOnlyFlags() }

func tokenizeCommandLine(commandLine string) ([]string, bool) {
	t := &cmdLineTokenizer{}
	t.tokenize(commandLine)

	return t.parts, t.isNewArg
}
