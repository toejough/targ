package core

import (
	"fmt"
	"reflect"
	"strings"
)

// PrintCompletionScript prints a shell completion script for the given shell.
func PrintCompletionScript(shell, binName string) error {
	switch shell {
	case "bash":
		fmt.Printf(_bashCompletion, binName, binName, binName, binName)
	case zshShell:
		fmt.Printf(_zshCompletion, binName, binName, binName, binName, binName)
	case fishShell:
		fmt.Printf(_fishCompletion, binName, binName, binName, binName)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	return nil
}

// unexported constants.
const (
	fishShell = "fish"
	zshShell  = "zsh"
)

// unexported variables.
var (
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
	// targBooleanFlags are flags that don't take a value.
	targBooleanFlags = map[string]bool{
		"--no-cache": true,
		"--keep":     true,
		"--help":     true,
		"-h":         true,
		"--init":     true, // can also use --init=FILE syntax
	}
	// targExitEarlyFlags cause targ to exit without running commands.
	// Everything after these flags is consumed by them.
	targExitEarlyFlags = map[string]bool{
		"--alias": true, // takes NAME "CMD" [FILE]
	}
	// targFlagsWithValues are flags that consume the next argument as a value.
	targFlagsWithValues = map[string]bool{
		"--timeout":    true,
		"--completion": true,
	}
	// targGlobalFlags are flags valid at any command level.
	targGlobalFlags = []string{"--help", "--timeout"}
	// targRootOnlyFlags are flags only valid at root level (before any command).
	targRootOnlyFlags = []string{"--no-cache", "--keep", "--completion", "--init", "--alias"}
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

// completionState holds state for shell completion.
type completionState struct {
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

// followRemaining handles remaining args after parsing.
func (s *completionState) followRemaining(result parseResult) bool {
	if !s.singleRoot {
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

	s.currentNode = s.roots[0]
	s.processedArgs = result.remaining
	s.explicit = false
	s.atRoot = true

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
		printIfPrefix(s.currentNode.Name, s.prefix)
	}

	for name := range s.currentNode.Subcommands {
		printIfPrefix(name, s.prefix)
	}

	if s.currentNode.Parent != nil {
		for name := range s.currentNode.Parent.Subcommands {
			printIfPrefix(name, s.prefix)
		}
	}

	if !s.atRoot {
		printIfPrefix("^", s.prefix)
	}
}

// suggestCompletions outputs completion suggestions.
func (s *completionState) suggestCompletions() error {
	s.suggestCommands()

	done, err := s.suggestEnumValues()
	if err != nil || done {
		return err
	}

	if err := s.suggestFlagsIfNeeded(); err != nil {
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
		printIfPrefix(value, s.prefix)
	}

	return true, nil
}

// suggestFlagsIfNeeded suggests flags if prefix starts with - or is empty.
func (s *completionState) suggestFlagsIfNeeded() error {
	if !strings.HasPrefix(s.prefix, "-") && s.prefix != "" {
		return nil
	}

	return suggestFlags(s.chain, s.prefix, s.atRoot)
}

// suggestMatchingRoots suggests roots that match a partial prefix.
func (s *completionState) suggestMatchingRoots(partial string) {
	for _, r := range s.roots {
		printIfPrefix(r.Name, partial)
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
		printIfPrefix(value, s.prefix)
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

	if suggested, err := s.suggestPositionalEnums(); err != nil || suggested {
		return err
	}

	s.suggestRootsIfAllowed()

	return nil
}

// suggestRootsAndFlags suggests all roots and targ flags at root level.
func (s *completionState) suggestRootsAndFlags() {
	for _, r := range s.roots {
		printIfPrefix(r.Name, s.prefix)
	}

	for _, opt := range targGlobalFlags {
		printIfPrefix(opt, s.prefix)
	}

	for _, opt := range targRootOnlyFlags {
		printIfPrefix(opt, s.prefix)
	}
}

// suggestRootsIfAllowed suggests root commands if conditions are met.
func (s *completionState) suggestRootsIfAllowed() {
	if !s.allowRootSuggests || !s.positionalsComplete || strings.HasPrefix(s.prefix, "-") {
		return
	}

	for _, root := range s.roots {
		printIfPrefix(root.Name, s.prefix)
	}
}

// positionalCounter tracks state for counting positional arguments.
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

	if len(arg) == 2 {
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

func completionChain(node *commandNode, args []string) ([]commandInstance, error) {
	if node == nil {
		return nil, nil
	}

	chain, _, err := completionParse(node, args, true)

	return chain, err
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
		inst, err := nodeInstance(current)
		if err != nil {
			return nil, parseResult{}, err
		}

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

func doCompletion(roots []*commandNode, commandLine string) error {
	state, done := prepareCompletionState(roots, commandLine)
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
	if len(flag) == 2 {
		spec, ok := specs[flag]

		return ok && spec.TakesValue
	}

	return expectingGroupedShortFlagValue(flag, specs)
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

func hasExitEarlyFlagPrefix(arg string) bool {
	for flag := range targExitEarlyFlags {
		if strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}

	return false
}

func hasFlagValuePrefix(arg string, flags map[string]bool) bool {
	for flag := range flags {
		if strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}

	return false
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
func prepareCompletionState(roots []*commandNode, commandLine string) (*completionState, bool) {
	parts, isNewArg := tokenizeCommandLine(commandLine)
	if len(parts) == 0 {
		return nil, true
	}

	parts = parts[1:] // Remove binary name

	prefix, processedArgs := extractPrefixAndArgs(parts, isNewArg)
	processedArgs = skipTargFlags(processedArgs)

	return &completionState{
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
func printIfPrefix(name, prefix string) {
	if strings.HasPrefix(name, prefix) {
		fmt.Println(name)
	}
}

// skipTargFlags removes targ-level flags from the args for completion purposes.
// These flags are handled by the outer targ binary, not the bootstrap.
func skipTargFlags(args []string) []string {
	result := make([]string, 0, len(args))

	skip := false
	for _, arg := range args {
		if skip {
			skip = false
			continue
		}
		// Exit-early flags consume all remaining args
		if targExitEarlyFlags[arg] || hasExitEarlyFlagPrefix(arg) {
			break
		}
		// Flags that take a value - skip flag and next arg
		if targFlagsWithValues[arg] {
			skip = true
			continue
		}
		// Flags with --flag=value syntax
		if hasFlagValuePrefix(arg, targFlagsWithValues) {
			continue
		}
		// Boolean flags (may also have --flag=value syntax for some like --init)
		if targBooleanFlags[arg] || hasFlagValuePrefix(arg, targBooleanFlags) {
			continue
		}

		result = append(result, arg)
	}

	return result
}

// suggestCommandFlags suggests flags from command chain fields.
func suggestCommandFlags(chain []commandInstance, prefix string, seen map[string]bool) error {
	for _, current := range chain {
		err := suggestInstanceFlags(current, prefix, seen)
		if err != nil {
			return err
		}
	}

	return nil
}

// suggestFieldFlags suggests the long and short flags for a field.
func suggestFieldFlags(
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

	suggestFlag("--"+opts.Name, prefix, seen)

	if opts.Short != "" {
		suggestFlag("-"+opts.Short, prefix, seen)
	}

	return nil
}

// suggestFlag prints a single flag if it matches prefix and hasn't been seen.
func suggestFlag(flag, prefix string, seen map[string]bool) {
	if strings.HasPrefix(flag, prefix) && !seen[flag] {
		fmt.Println(flag)
		seen[flag] = true
	}
}

func suggestFlags(chain []commandInstance, prefix string, atRoot bool) error {
	if len(chain) == 0 {
		return nil
	}

	seen := map[string]bool{}

	err := suggestCommandFlags(chain, prefix, seen)
	if err != nil {
		return err
	}

	suggestMatchingFlags(targGlobalFlags, prefix, seen)

	if atRoot {
		suggestMatchingFlags(targRootOnlyFlags, prefix, seen)
	}

	return nil
}

// suggestInstanceFlags suggests flags from a single command instance.
func suggestInstanceFlags(current commandInstance, prefix string, seen map[string]bool) error {
	if current.node == nil || current.node.Type == nil {
		return nil
	}

	typ := current.node.Type
	for i := range typ.NumField() {
		err := suggestFieldFlags(current, typ.Field(i), prefix, seen)
		if err != nil {
			return err
		}
	}

	return nil
}

// suggestMatchingFlags prints flags that match the prefix.
func suggestMatchingFlags(flags []string, prefix string, seen map[string]bool) {
	for _, flag := range flags {
		suggestFlag(flag, prefix, seen)
	}
}

func tokenizeCommandLine(commandLine string) ([]string, bool) {
	t := &cmdLineTokenizer{}
	t.tokenize(commandLine)

	return t.parts, t.isNewArg
}
