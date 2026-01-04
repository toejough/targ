package targ

import (
	"flag"
	"fmt"
	"io"
	"reflect"
	"strings"
)

func doCompletion(roots []*CommandNode, commandLine string) error {
	// 1. Tokenize the command line
	// The commandLine includes the binary name. e.g. "myapp build -t"
	parts, isNewArg := tokenizeCommandLine(commandLine)
	if len(parts) == 0 {
		return nil
	}

	// Remove binary name
	parts = parts[1:]

	// Traverse to find the current node and the word being completed
	// We want to complete the LAST part.

	var prefix string
	var processedArgs []string

	if !isNewArg && len(parts) > 0 {
		prefix = parts[len(parts)-1]
		processedArgs = parts[:len(parts)-1]
	} else {
		prefix = ""
		processedArgs = parts
	}

	// Traverse
	var currentNode *CommandNode
	singleRoot := len(roots) == 1
	atRoot := true

	// Find root
	if singleRoot {
		currentNode = roots[0]
	} else {
		if len(processedArgs) > 0 {
			rootName := processedArgs[0]
			for _, r := range roots {
				if strings.EqualFold(r.Name, rootName) {
					currentNode = r
					break
				}
			}
			processedArgs = processedArgs[1:]
			atRoot = false
		} else {
			// We are at root level
			// Suggest roots
			for _, r := range roots {
				if strings.HasPrefix(r.Name, prefix) {
					fmt.Println(r.Name)
				}
			}
			return nil
		}
	}

	if currentNode == nil {
		return nil
	}

	// Walk subcommands (stop when we hit flags or end-of-flags)
	for len(processedArgs) > 0 {
		subName := processedArgs[0]
		if subName == "--" || strings.HasPrefix(subName, "-") {
			break
		}
		if sub, ok := currentNode.Subcommands[subName]; ok {
			currentNode = sub
			processedArgs = processedArgs[1:]
			atRoot = false
		} else {
			// Unknown path, cannot complete further
			return nil
		}
	}

	// Now we are at currentNode, and we need to suggest based on prefix

	// 1. Suggest Subcommands
	for name := range currentNode.Subcommands {
		if strings.HasPrefix(name, prefix) {
			fmt.Println(name)
		}
	}

	// 2. Suggest Flags
	// We need to look at fields again.
	// Note: We need to recreate the flag set logic or reuse parsing?
	// Reusing parsing is hard because it consumes args.
	// We just want to inspect the struct fields.

	// Check if prefix starts with "-"
	chain, err := completionChain(currentNode, processedArgs)
	if err != nil {
		return err
	}
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
		if err := suggestFlags(chain, prefix, atRoot); err != nil {
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
	posIndex, err := positionalIndex(currentNode, processedArgs)
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
		return nil
	}
	if fields[posIndex].Opts.Enum == "" {
		return nil
	}
	values = strings.Split(fields[posIndex].Opts.Enum, "|")
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			fmt.Println(value)
		}
	}
	return nil
}

func tokenizeCommandLine(commandLine string) ([]string, bool) {
	var parts []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	isNewArg := false

	for i := 0; i < len(commandLine); i++ {
		ch := commandLine[i]
		if escaped {
			current.WriteByte(ch)
			escaped = false
			isNewArg = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			isNewArg = false
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			isNewArg = false
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			isNewArg = false
			continue
		}
		if (ch == ' ' || ch == '\t' || ch == '\n') && !inSingle && !inDouble {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			isNewArg = true
			continue
		}
		current.WriteByte(ch)
		isNewArg = false
	}

	if escaped {
		current.WriteByte('\\')
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	if inSingle || inDouble {
		isNewArg = false
	}
	return parts, isNewArg
}

func suggestFlags(chain []commandInstance, prefix string, includeCompletion bool) error {
	if len(chain) == 0 {
		return nil
	}

	seen := map[string]bool{}
	for _, current := range chain {
		if current.node == nil || current.node.Type == nil {
			continue
		}
		inst := current.value
		typ := current.node.Type
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			opts, ok, err := tagOptionsForField(inst, field)
			if err != nil {
				return err
			}
			if !ok || opts.Kind != TagKindFlag {
				continue
			}

			name := opts.Name
			shortName := opts.Short

			longFlag := "--" + name
			if strings.HasPrefix(longFlag, prefix) && !seen[longFlag] {
				fmt.Println(longFlag)
				seen[longFlag] = true
			}
			if shortName != "" {
				shortFlag := "-" + shortName
				if strings.HasPrefix(shortFlag, prefix) && !seen[shortFlag] {
					fmt.Println(shortFlag)
					seen[shortFlag] = true
				}
			}
		}
	}

	if includeCompletion {
		comp := "--completion"
		if strings.HasPrefix(comp, prefix) {
			fmt.Println(comp)
		}
	}
	return nil
}

func enumValuesForArg(chain []commandInstance, args []string, prefix string, isNewArg bool) ([]string, bool, error) {
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

	previous := ""
	if isNewArg {
		if len(args) == 0 {
			return nil, false, nil
		}
		previous = args[len(args)-1]
	} else {
		if len(args) == 0 {
			return nil, false, nil
		}
		previous = args[len(args)-1]
		if strings.HasPrefix(prefix, "-") {
			return nil, false, nil
		}
	}

	if values, ok := enumByFlag[previous]; ok {
		return values, true, nil
	}
	return nil, false, nil
}

type completionFlagSpec struct {
	TakesValue bool
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
			specs["--"+opts.Name] = completionFlagSpec{TakesValue: takesValue}
			if opts.Short != "" {
				specs["-"+opts.Short] = completionFlagSpec{TakesValue: takesValue}
			}
		}
	}
	return specs, nil
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
		if strings.Contains(last, "=") {
			return false
		}
		if spec, ok := specs[last]; ok && spec.TakesValue {
			return true
		}
		return false
	}
	if strings.HasPrefix(last, "-") && len(last) == 2 {
		if spec, ok := specs[last]; ok && spec.TakesValue {
			return true
		}
		return false
	}
	if strings.HasPrefix(last, "-") && len(last) > 2 {
		group := strings.TrimPrefix(last, "-")
		for i, ch := range group {
			spec, ok := specs["-"+string(ch)]
			if !ok {
				continue
			}
			if spec.TakesValue {
				return i == len(group)-1
			}
		}
	}
	return false
}

type positionalField struct {
	Field reflect.StructField
	Opts  TagOptions
}

func positionalFields(node *CommandNode, inst reflect.Value) ([]positionalField, error) {
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

func positionalIndex(node *CommandNode, args []string) (int, error) {
	chain, err := completionChain(node, args)
	if err != nil {
		return 0, err
	}
	specs, err := completionFlagSpecs(chain)
	if err != nil {
		return 0, err
	}
	count := 0
	afterDash := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if afterDash {
			count++
			continue
		}
		if arg == "--" {
			afterDash = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if eq := strings.Index(arg, "="); eq != -1 {
				continue
			}
			if spec, ok := specs[arg]; ok && spec.TakesValue {
				if i+1 < len(args) {
					i++
				}
			}
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			if len(arg) == 2 {
				if spec, ok := specs[arg]; ok && spec.TakesValue {
					if i+1 < len(args) {
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

func completionChain(node *CommandNode, args []string) ([]commandInstance, error) {
	chainNodes := nodeChain(node)
	chain := make([]commandInstance, 0, len(chainNodes))
	for _, current := range chainNodes {
		inst, err := nodeInstance(current)
		if err != nil {
			return nil, err
		}
		chain = append(chain, commandInstance{node: current, value: inst})
	}
	if len(chain) == 0 {
		return nil, nil
	}
	fs := flag.NewFlagSet(node.Name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	specs, _, err := registerChainFlags(fs, chain)
	if err != nil {
		return nil, err
	}
	expandedArgs, err := expandShortFlagGroups(args, specs)
	if err != nil {
		return nil, err
	}
	_ = fs.Parse(expandedArgs)
	return chain, nil
}

// PrintCompletionScript prints a shell completion script for the given shell.
func PrintCompletionScript(shell string, binName string) error {
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

// Simplified templates
var _bashCompletion = `
_%s_completion() {
    local request="${COMP_LINE}"
    local completions
    completions=$(%s __complete "$request")
    
    COMPREPLY=( $(compgen -W "$completions" -- "${COMP_WORDS[COMP_CWORD]}") )
}
complete -F _%s_completion %s
`

var _zshCompletion = `
#compdef %s

_%s_completion() {
    local request="${words[*]}"
    local completions
    completions=("${(@f)$(%s __complete "$request")}")
    
    compadd -a completions
}
compdef _%s_completion %s
`

var _fishCompletion = `
function __%s_complete
    set -l request (commandline -cp)
    %s __complete "$request"
end
complete -c %s -a "(__%s_complete)" -f
`
