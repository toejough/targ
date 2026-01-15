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
	case "zsh":
		fmt.Printf(_zshCompletion, binName, binName, binName, binName, binName)
	case "fish":
		fmt.Printf(_fishCompletion, binName, binName, binName, binName)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	return nil
}

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

type positionalField struct {
	Field reflect.StructField
	Opts  TagOptions
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
		for i := 0; i < current.node.Type.NumField(); i++ {
			field := current.node.Type.Field(i)

			opts, ok, err := tagOptionsForField(inst, field)
			if err != nil {
				return nil, err
			}

			if !ok || opts.Kind != TagKindFlag {
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
	// 1. Tokenize the command line
	// The commandLine includes the binary name. e.g. "myapp build -t"
	parts, isNewArg := tokenizeCommandLine(commandLine)
	if len(parts) == 0 {
		return nil
	}

	// Remove binary name
	parts = parts[1:]

	var (
		prefix        string
		processedArgs []string
	)

	if !isNewArg && len(parts) > 0 {
		prefix = parts[len(parts)-1]
		processedArgs = parts[:len(parts)-1]
	} else {
		prefix = ""
		processedArgs = parts
	}

	// Skip targ-level flags (--no-cache, --keep, --timeout, --completion, --help)
	processedArgs = skipTargFlags(processedArgs)
	// Note: prefix might have been a targ flag value, but we handle this
	// by recalculating context from processedArgs below.

	// Resolve current command context.
	var currentNode *commandNode

	singleRoot := len(roots) == 1
	atRoot := true
	allowRootSuggestions := len(roots) > 1

	var positionalsComplete bool

	if singleRoot {
		currentNode = roots[0]
	} else {
		if len(processedArgs) == 0 {
			// At root level, suggest both commands and flags
			for _, r := range roots {
				if strings.HasPrefix(r.Name, prefix) {
					fmt.Println(r.Name)
				}
			}
			// Also suggest targ flags at root level
			for _, opt := range targGlobalFlags {
				if strings.HasPrefix(opt, prefix) {
					fmt.Println(opt)
				}
			}

			for _, opt := range targRootOnlyFlags {
				if strings.HasPrefix(opt, prefix) {
					fmt.Println(opt)
				}
			}

			return nil
		}

		rootName := processedArgs[0]
		for _, r := range roots {
			if strings.EqualFold(r.Name, rootName) {
				currentNode = r
				break
			}
		}

		if currentNode == nil {
			// If no root matched, it might be a partial prefix - suggest matching roots
			for _, r := range roots {
				if strings.HasPrefix(r.Name, processedArgs[0]) {
					fmt.Println(r.Name)
				}
			}

			return nil
		}

		processedArgs = processedArgs[1:]
		atRoot = false
	}

	explicit := !singleRoot

	var chain []commandInstance

	for {
		nextChain, result, err := completionParse(currentNode, processedArgs, explicit)
		if err != nil {
			return nil
		}

		chain = nextChain

		positionalsComplete = result.positionalsComplete
		if result.subcommand != nil {
			currentNode = result.subcommand
			processedArgs = result.remaining
			explicit = true
			atRoot = false

			continue
		}

		if len(result.remaining) > 0 {
			if !singleRoot {
				nextRoot := findCompletionRoot(roots, result.remaining[0])
				if nextRoot == nil {
					return nil
				}

				currentNode = nextRoot
				processedArgs = result.remaining[1:]
				explicit = true
				atRoot = false

				continue
			}

			currentNode = roots[0]
			processedArgs = result.remaining
			explicit = false
			atRoot = true

			continue
		}

		break
	}

	// Now we are at currentNode, and we need to suggest based on prefix

	// 0. For single root at root level, suggest the root command name itself
	if singleRoot && atRoot && strings.HasPrefix(currentNode.Name, prefix) {
		fmt.Println(currentNode.Name)
	}

	// 1. Suggest Subcommands (children)
	for name := range currentNode.Subcommands {
		if strings.HasPrefix(name, prefix) {
			fmt.Println(name)
		}
	}

	// 2. Suggest Siblings (parent's subcommands) for implicit sibling resolution
	if currentNode.Parent != nil {
		for name := range currentNode.Parent.Subcommands {
			if strings.HasPrefix(name, prefix) {
				fmt.Println(name)
			}
		}
	}

	// 3. Suggest ^ for root reset when not at root
	if !atRoot && strings.HasPrefix("^", prefix) {
		fmt.Println("^")
	}

	// 4. Suggest Flags
	// Check if prefix starts with "-"
	values, valuesOK, err := enumValuesForArg(chain, processedArgs, prefix, isNewArg)
	if err != nil {
		return err
	}

	if valuesOK {
		for _, value := range values {
			if strings.HasPrefix(value, prefix) {
				fmt.Println(value)
			}
		}

		return nil
	}

	if strings.HasPrefix(prefix, "-") || prefix == "" {
		err := suggestFlags(chain, prefix, atRoot)
		if err != nil {
			return err
		}
	}

	if strings.HasPrefix(prefix, "-") {
		return nil
	}

	specs, err := completionFlagSpecs(chain)
	if err != nil {
		return err
	}

	if expectingFlagValue(processedArgs, specs) {
		return nil
	}

	posIndex, err := positionalIndex(currentNode, processedArgs, chain)
	if err != nil {
		return err
	}

	if len(chain) == 0 {
		return nil
	}

	fields, err := positionalFields(chain[len(chain)-1].node, chain[len(chain)-1].value)
	if err != nil {
		return err
	}

	if posIndex >= len(fields) {
		goto maybeSuggestRoots
	}

	if fields[posIndex].Opts.Enum == "" {
		goto maybeSuggestRoots
	}

	values = strings.Split(fields[posIndex].Opts.Enum, "|")
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			fmt.Println(value)
		}
	}

	return nil

maybeSuggestRoots:
	if !allowRootSuggestions || !positionalsComplete || strings.HasPrefix(prefix, "-") {
		return nil
	}

	for _, root := range roots {
		if strings.HasPrefix(root.Name, prefix) {
			fmt.Println(root.Name)
		}
	}

	return nil
}

func enumValuesForArg(
	chain []commandInstance,
	args []string,
	prefix string,
	isNewArg bool,
) ([]string, bool, error) {
	enumByFlag := map[string][]string{}

	for _, current := range chain {
		if current.node == nil || current.node.Type == nil {
			continue
		}

		inst := current.value
		for i := 0; i < current.node.Type.NumField(); i++ {
			field := current.node.Type.Field(i)

			opts, ok, err := tagOptionsForField(inst, field)
			if err != nil {
				return nil, false, err
			}

			if !ok || opts.Kind != TagKindFlag {
				continue
			}

			name := opts.Name
			shortName := opts.Short

			enumValues := []string{}
			if opts.Enum != "" {
				enumValues = strings.Split(opts.Enum, "|")
			}

			if len(enumValues) == 0 {
				continue
			}

			key := "--" + name
			if _, exists := enumByFlag[key]; !exists {
				enumByFlag[key] = enumValues
			}

			if shortName != "" {
				key = "-" + shortName
				if _, exists := enumByFlag[key]; !exists {
					enumByFlag[key] = enumValues
				}
			}
		}
	}

	if len(enumByFlag) == 0 {
		return nil, false, nil
	}

	if len(args) == 0 {
		return nil, false, nil
	}

	previous := args[len(args)-1]

	if !isNewArg && strings.HasPrefix(prefix, "-") {
		return nil, false, nil
	}

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

	var fields []positionalField

	for i := 0; i < node.Type.NumField(); i++ {
		field := node.Type.Field(i)

		opts, ok, err := tagOptionsForField(inst, field)
		if err != nil {
			return nil, err
		}

		if !ok || opts.Kind != TagKindPositional {
			continue
		}

		fields = append(fields, positionalField{Field: field, Opts: opts})
	}

	return fields, nil
}

func positionalIndex(node *commandNode, args []string, chain []commandInstance) (int, error) {
	specs, err := completionFlagSpecs(chain)
	if err != nil {
		return 0, err
	}

	count := 0

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			continue
		}

		if strings.HasPrefix(arg, "--") {
			if found := strings.Contains(arg, "="); found {
				continue
			}

			if spec, ok := specs[arg]; ok && spec.TakesValue {
				if spec.Variadic {
					for i+1 < len(args) {
						next := args[i+1]
						if next == "--" || strings.HasPrefix(next, "-") {
							break
						}

						i++
					}
				} else if i+1 < len(args) {
					i++
				}
			}

			continue
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			if strings.Contains(arg, "=") {
				continue
			}

			if len(arg) == 2 {
				if spec, ok := specs[arg]; ok && spec.TakesValue {
					if spec.Variadic {
						for i+1 < len(args) {
							next := args[i+1]
							if next == "--" || strings.HasPrefix(next, "-") {
								break
							}

							i++
						}
					} else if i+1 < len(args) {
						i++
					}
				}

				continue
			}

			group := strings.TrimPrefix(arg, "-")
			consumed := false

			for idx, ch := range group {
				spec, ok := specs["-"+string(ch)]
				if !ok {
					continue
				}

				if spec.TakesValue {
					if idx == len(group)-1 && i+1 < len(args) {
						i++
					}

					consumed = true

					break
				}
			}

			if consumed {
				continue
			}

			continue
		}

		count++
	}

	return count, nil
}

// skipTargFlags removes targ-level flags from the args for completion purposes.
// These flags are handled by the outer targ binary, not the bootstrap.
func skipTargFlags(args []string) []string {
	var result []string

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
	opts, ok, err := tagOptionsForField(current.value, field)
	if err != nil {
		return err
	}

	if !ok || opts.Kind != TagKindFlag {
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
	for i := 0; i < typ.NumField(); i++ {
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
