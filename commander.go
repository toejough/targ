package commander

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
	"unicode"
)

// Run executes the CLI.
func Run(targets ...interface{}) {
	roots := []*CommandNode{}
	for _, t := range targets {
		node, err := parseStruct(t)
		if err != nil {
			fmt.Printf("Error parsing target: %v\n", err)
			continue
		}
		roots = append(roots, node)
	}

	if len(os.Args) < 2 {
		printUsage(roots)
		return
	}

	args := os.Args[1:]

	// Handle global help
	if args[0] == "-h" || args[0] == "--help" {
		printUsage(roots)
		return
	}

	// Find matching root
	var matched *CommandNode
	for _, root := range roots {
		if strings.EqualFold(root.Name, args[0]) {
			matched = root
			break
		}
	}

	if matched == nil {
		fmt.Printf("Unknown command: %s\n", args[0])
		printUsage(roots)
		os.Exit(1)
	}

	// Execute the matched root
	if err := matched.execute(args[1:]); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage(nodes []*CommandNode) {
	fmt.Println("Usage: <command> [args]")
	fmt.Println("\nAvailable commands:")
	for _, node := range nodes {
		fmt.Printf("  %s\n", node.Name)
		// Todo: print subcommands?
	}
}

type CommandNode struct {
	Name        string
	Type        reflect.Type
	Value       reflect.Value // The struct instance
	Subcommands map[string]*CommandNode
	RunMethod   reflect.Value
	Description string
}

func parseStruct(t interface{}) (*CommandNode, error) {
	v := reflect.ValueOf(t)
	typ := v.Type()
	
	// Handle pointer
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		v = v.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %v", typ.Kind())
	}

	name := camelToKebab(typ.Name())
	
	node := &CommandNode{
		Name:        name,
		Type:        typ,
		Value:       v,
		Subcommands: make(map[string]*CommandNode),
	}

	// 1. Look for Run method on the *pointer* to the struct
	// Check for Run method on Pointer type
	ptrType := reflect.PtrTo(typ)
	_, hasRun := ptrType.MethodByName("Run")
	if hasRun {
		node.RunMethod = reflect.Value{} // Marker
	}

	// 2. Look for fields with `commander:"subcommand"`
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("commander")
		if strings.Contains(tag, "subcommand") {
			// This field is a subcommand
			// Recurse
			
			// We need an instance of the field type to parse it?
			// `parseStruct` expects interface value.
			// We can create a zero value.
			
			fieldType := field.Type
			// If pointer, get elem
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}
			
			zeroVal := reflect.New(fieldType).Interface() // This is *Struct
			subNode, err := parseStruct(zeroVal)
			if err != nil {
				return nil, err
			}
			
			// Override name based on field or tag
			// The node comes with a default name based on its Type, but the Field name usually takes precedence
			// unless the tag explicitly sets a name.
			
			nameOverride := ""
			parts := strings.Split(tag, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "name=") {
					nameOverride = strings.TrimPrefix(p, "name=")
				} else if strings.HasPrefix(p, "subcommand=") {
					nameOverride = strings.TrimPrefix(p, "subcommand=")
				}
			}
			
			if nameOverride != "" {
				subNode.Name = nameOverride
			} else {
				subNode.Name = camelToKebab(field.Name)
			}
			
			node.Subcommands[subNode.Name] = subNode
		}
	}

	// 3. Look for legacy method-based subcommands (optional, keeping for compat if desired)
	// For now, let's focus on the field based approach as requested.

	return node, nil
}

func (n *CommandNode) execute(args []string) error {
	// 1. Use existing value if possible, otherwise create new
	var inst reflect.Value
	if n.Value.IsValid() && n.Value.CanAddr() {
		inst = n.Value
	} else {
		// Create new pointer to make it addressable
		instPtr := reflect.New(n.Type)
		inst = instPtr.Elem()
		
		// Copy existing value if we have one
		if n.Value.IsValid() {
			inst.Set(n.Value)
		}
	}
	
	// 2. Parse flags for THIS level
	fs := flag.NewFlagSet(n.Name, flag.ContinueOnError)
	
	// Map fields to flags
	for i := 0; i < n.Type.NumField(); i++ {
		field := n.Type.Field(i)
		tag := field.Tag.Get("commander")
		if strings.Contains(tag, "subcommand") {
			continue
		}
		
		name := strings.ToLower(field.Name)
		usage := ""
		defaultValue := ""
		
		if tag != "" {
			parts := strings.Split(tag, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "name=") {
					name = strings.TrimPrefix(p, "name=")
				} else if strings.HasPrefix(p, "env=") {
					envVar := strings.TrimPrefix(p, "env=")
					if val, ok := os.LookupEnv(envVar); ok {
						defaultValue = val
					}
				}
				// desc handling...
			}
		}

		// Apply default from env if not zero? 
		// Actually flag sets default.
		
		fieldVal := inst.Field(i)
		
		switch field.Type.Kind() {
		case reflect.String:
			fs.StringVar(fieldVal.Addr().Interface().(*string), name, defaultValue, usage)
		case reflect.Int:
			intVal := 0
			if defaultValue != "" {
				fmt.Sscanf(defaultValue, "%d", &intVal)
			}
			fs.IntVar(fieldVal.Addr().Interface().(*int), name, intVal, usage)
		case reflect.Bool:
			// Bool env var handling
			boolVal := false
			if defaultValue == "true" || defaultValue == "1" {
				boolVal = true
			}
			fs.BoolVar(fieldVal.Addr().Interface().(*bool), name, boolVal, usage)
		}
	}
	
	if err := fs.Parse(args); err != nil {
		return err
	}
	
	// Check required (simple check)
	// We'd need to track which were set. `flag` package doesn't make this easy without `Visit`.
	// Skipping precise required check for brevity in this iteration, but keeping logic structure.
	
	remaining := fs.Args()
	
	// 3. Check for subcommands in remaining args
	if len(remaining) > 0 {
		subName := remaining[0]
		// fmt.Printf("Debug: searching for '%s' in %v\n", subName, n.Subcommands)
		if sub, ok := n.Subcommands[subName]; ok {
			// Found subcommand. 
			// We need to populate the field in `inst` with the subcommand instance?
			// `sub` is a CommandNode. It has `Value` which is the zero value created in parsing.
			// When `sub.execute` runs, it will populate `sub.Value`.
			// We should assign `sub.Value` (pointer?) to the field in `inst`.
			
			// Find the field for this subcommand
			// Note: We need to check for name override in tag here too to match subName
			for i := 0; i < n.Type.NumField(); i++ {
				field := n.Type.Field(i)
				tag := field.Tag.Get("commander")
				if strings.Contains(tag, "subcommand") {
					name := camelToKebab(field.Name)
					// Check override
					parts := strings.Split(tag, ",")
					for _, p := range parts {
						p = strings.TrimSpace(p)
						if strings.HasPrefix(p, "name=") {
							name = strings.TrimPrefix(p, "name=")
						} else if strings.HasPrefix(p, "subcommand=") {
							name = strings.TrimPrefix(p, "subcommand=")
						}
					}
					
					if name == subName {
						// Assign sub.Value to this field.
						if sub.Value.CanAddr() {
							inst.Field(i).Set(sub.Value.Addr())
						}
						break
					}
				}
			}
			
			return sub.execute(remaining[1:])
		}
	}
	
	// 4. Handle Positional Args
	posArgIdx := 0
	for i := 0; i < n.Type.NumField(); i++ {
		field := n.Type.Field(i)
		tag := field.Tag.Get("commander")
		if strings.Contains(tag, "positional") {
			if posArgIdx >= len(remaining) {
				break
			}
			val := remaining[posArgIdx]
			
			fVal := inst.Field(i)
			switch fVal.Kind() {
			case reflect.String:
				fVal.SetString(val)
			case reflect.Int:
				var i int64
				fmt.Sscanf(val, "%d", &i)
				fVal.SetInt(i)
			}
			posArgIdx++
		}
	}

	// 5. Execute Run
	// We need to call Run on the Pointer to inst?
	// inst is addressable.
	method := inst.Addr().MethodByName("Run")
	if method.IsValid() {
		if method.Type().NumIn() == 0 {
			method.Call(nil)
			return nil
		}
	}
	
	if len(n.Subcommands) > 0 {
		// Just list subcommands if we didn't run anything
		fmt.Printf("Command '%s' requires a subcommand:\n", n.Name)
		for name := range n.Subcommands {
			fmt.Println(" -", name)
		}
		return nil
	}
	
	return fmt.Errorf("command %s is not runnable (no Run method)", n.Name)
}

func camelToKebab(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('-')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}
