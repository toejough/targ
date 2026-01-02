package commander

import (
	"fmt"
	"strings"
)

func doCompletion(roots []*CommandNode, commandLine string) {
	// 1. Tokenize the command line
	// The commandLine includes the binary name. e.g. "myapp build -t"
	parts, isNewArg := tokenizeCommandLine(commandLine)
	if len(parts) == 0 {
		return
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
			// Also suggest "completion"
			if strings.HasPrefix("completion", prefix) {
				fmt.Println("completion")
			}
			return
		}
	}

	if currentNode == nil {
		return
	}

	// Walk subcommands
	for len(processedArgs) > 0 {
		subName := processedArgs[0]
		if sub, ok := currentNode.Subcommands[subName]; ok {
			currentNode = sub
			processedArgs = processedArgs[1:]
			atRoot = false
		} else {
			// Unknown path, cannot complete further
			return
		}
	}

	// Now we are at currentNode, and we need to suggest based on prefix

	// 1. Suggest Subcommands
	for name := range currentNode.Subcommands {
		if strings.HasPrefix(name, prefix) {
			fmt.Println(name)
		}
	}

	if atRoot && strings.HasPrefix("completion", prefix) {
		fmt.Println("completion")
	}

	// 2. Suggest Flags
	// We need to look at fields again.
	// Note: We need to recreate the flag set logic or reuse parsing?
	// Reusing parsing is hard because it consumes args.
	// We just want to inspect the struct fields.

	// Check if prefix starts with "-"
	if strings.HasPrefix(prefix, "-") || prefix == "" {
		suggestFlags(currentNode, prefix)
	}
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

func suggestFlags(node *CommandNode, prefix string) {
	if node.Type == nil {
		return
	}

	typ := node.Type
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("commander")
		if strings.Contains(tag, "subcommand") {
			continue
		}

		name := strings.ToLower(field.Name)
		// Check override
		parts := strings.Split(tag, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "name=") {
				name = strings.TrimPrefix(p, "name=")
			}
		}

		flagName := "-" + name
		if strings.HasPrefix(flagName, prefix) {
			fmt.Println(flagName)
		}
	}
}

// generateCompletionScript prints the shell script.
func generateCompletionScript(shell string, binName string) {
	switch shell {
	case "bash":
		fmt.Printf(_bashCompletion, binName, binName)
	case "zsh":
		fmt.Printf(_zshCompletion, binName, binName)
	case "fish":
		fmt.Printf(_fishCompletion, binName, binName)
	default:
		fmt.Printf("Unsupported shell: %s. Supported: bash, zsh, fish\n", shell)
	}
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
