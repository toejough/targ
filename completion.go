package commander

import (
	"fmt"
	"strings"
)

func doCompletion(roots []*CommandNode, commandLine string) {
	// 1. Tokenize the command line
	// Note: simplistic split for now. 
	// The commandLine includes the binary name. e.g. "myapp build -t"
	parts := strings.Fields(commandLine)
	if len(parts) == 0 {
		return
	}
	
	// Remove binary name
	parts = parts[1:]
	
	// Traverse to find the current node and the word being completed
	// We want to complete the LAST part.
	
	// If the line ends with a space, we are expecting a NEW argument.
	// If it doesn't, we are completing the CURRENT argument.
	
	isNewArg := strings.HasSuffix(commandLine, " ")
	
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
	
	// Find root
	if len(processedArgs) > 0 {
		rootName := processedArgs[0]
		for _, r := range roots {
			if strings.EqualFold(r.Name, rootName) {
				currentNode = r
				break
			}
		}
		processedArgs = processedArgs[1:]
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
	
	if currentNode == nil {
		return
	}
	
	// Walk subcommands
	for len(processedArgs) > 0 {
		subName := processedArgs[0]
		if sub, ok := currentNode.Subcommands[subName]; ok {
			currentNode = sub
			processedArgs = processedArgs[1:]
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

func suggestFlags(node *CommandNode, prefix string) {
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
