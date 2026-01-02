package commander

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

// Run executes the CLI.
func Run(targets ...interface{}) {
	RunWithOptions(RunOptions{AllowDefault: true}, targets...)
}

// RunOptions controls runtime behavior for RunWithOptions.
type RunOptions struct {
	AllowDefault bool
}

// RunWithOptions executes the CLI with configurable behavior.
func RunWithOptions(opts RunOptions, targets ...interface{}) {
	runWithEnv(osRunEnv{}, opts, targets...)
}

func printUsage(nodes []*CommandNode) {
	fmt.Println("Usage: <command> [args]")
	fmt.Println("\nAvailable commands:")

	for _, node := range nodes {
		printCommandSummary(node, "  ")
	}
}

func printCommandSummary(node *CommandNode, indent string) {
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
		printCommandSummary(sub, indent+"  ")
	}
}

type CommandNode struct {
	Name        string
	Type        reflect.Type
	Value       reflect.Value // The struct instance
	Func        reflect.Value // Niladic function target
	Subcommands map[string]*CommandNode
	RunMethod   reflect.Value
	Description string
}

type requiredFlagGroup struct {
	names   []string
	fromEnv bool
	display string
}

func parseStruct(t interface{}) (*CommandNode, error) {
	if t == nil {
		return nil, fmt.Errorf("nil target")
	}
	v := reflect.ValueOf(t)
	typ := v.Type()

	// Handle pointer
	if typ.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, fmt.Errorf("nil pointer target")
		}
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
	runMethod, hasRun := ptrType.MethodByName("Run")
	if hasRun {
		node.RunMethod = reflect.Value{} // Marker
		// Extract description from doc string
		doc := getMethodDoc(runMethod)
		if doc != "" {
			node.Description = strings.TrimSpace(doc)
		}
	}

	// 2. Look for fields with `commander:"subcommand"`
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("commander")
		if strings.Contains(tag, "subcommand") {
			// This field is a subcommand
			// Recurse

			fieldType := field.Type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			var subNode *CommandNode
			if fieldType.Kind() == reflect.Func {
				if err := validateNiladicFuncType(field.Type); err != nil {
					return nil, err
				}
				subNode = &CommandNode{
					Func:        reflect.Zero(field.Type),
					Subcommands: make(map[string]*CommandNode),
				}
			} else {
				// We need an instance of the field type to parse it.
				zeroVal := reflect.New(fieldType).Interface() // This is *Struct
				var err error
				subNode, err = parseStruct(zeroVal)
				if err != nil {
					return nil, err
				}
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
	if n.Func.IsValid() {
		if n.Func.Kind() == reflect.Func && n.Func.IsNil() {
			return fmt.Errorf("command %s function is nil", n.Name)
		}
		if len(args) > 0 {
			if args[0] == "-h" || args[0] == "--help" {
				printCommandHelp(n)
				return nil
			}
			return fmt.Errorf("unknown arguments: %v", args)
		}
		if err := validateNiladicFuncType(n.Func.Type()); err != nil {
			return fmt.Errorf("command %s %v", n.Name, err)
		}
		out := n.Func.Call(nil)
		if len(out) == 1 {
			if err, ok := out[0].Interface().(error); ok && err != nil {
				return err
			}
		}
		return nil
	}

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
	fs.Usage = func() { printCommandHelp(n) }

	requiredFlags := []requiredFlagGroup{}

	// Map fields to flags
	for i := 0; i < n.Type.NumField(); i++ {
		field := n.Type.Field(i)
		tag := field.Tag.Get("commander")
		if strings.Contains(tag, "subcommand") || strings.Contains(tag, "positional") {
			continue
		}

		name := strings.ToLower(field.Name)
		usage := ""
		defaultValue := ""
		shortName := ""
		required := false
		envSet := false

		if tag != "" {
			parts := strings.Split(tag, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "name=") {
					name = strings.TrimPrefix(p, "name=")
				} else if strings.HasPrefix(p, "short=") {
					shortName = strings.TrimPrefix(p, "short=")
				} else if strings.HasPrefix(p, "env=") {
					envVar := strings.TrimPrefix(p, "env=")
					if val, ok := os.LookupEnv(envVar); ok && val != "" {
						defaultValue = val
						envSet = true
					}
				} else if p == "required" {
					required = true
				}
				// desc handling...
			}
		}

		fieldVal := inst.Field(i)
		if !envSet {
			switch field.Type.Kind() {
			case reflect.String:
				defaultValue = fieldVal.String()
			case reflect.Int:
				defaultValue = fmt.Sprintf("%d", fieldVal.Int())
			case reflect.Bool:
				if fieldVal.Bool() {
					defaultValue = "true"
				} else {
					defaultValue = "false"
				}
			}
		}

		switch field.Type.Kind() {
		case reflect.String:
			fs.StringVar(fieldVal.Addr().Interface().(*string), name, defaultValue, usage)
			if shortName != "" {
				fs.StringVar(fieldVal.Addr().Interface().(*string), shortName, defaultValue, usage)
			}
		case reflect.Int:
			intVal := 0
			if defaultValue != "" {
				fmt.Sscanf(defaultValue, "%d", &intVal)
			}
			fs.IntVar(fieldVal.Addr().Interface().(*int), name, intVal, usage)
			if shortName != "" {
				fs.IntVar(fieldVal.Addr().Interface().(*int), shortName, intVal, usage)
			}
		case reflect.Bool:
			// Bool env var handling
			boolVal := false
			if defaultValue == "true" || defaultValue == "1" {
				boolVal = true
			}
			fs.BoolVar(fieldVal.Addr().Interface().(*bool), name, boolVal, usage)
			if shortName != "" {
				fs.BoolVar(fieldVal.Addr().Interface().(*bool), shortName, boolVal, usage)
			}
		}

		if required {
			names := []string{name}
			displayParts := []string{"--" + name}
			if shortName != "" {
				names = append(names, shortName)
				displayParts = append(displayParts, "-"+shortName)
			}
			requiredFlags = append(requiredFlags, requiredFlagGroup{
				names:   names,
				fromEnv: envSet,
				display: strings.Join(displayParts, "/"),
			})
		}
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			// printCommandHelp(n) // We call printCommandHelp manually later if we want custom format,
			// but flag package prints its own help before returning ErrHelp.
			// To suppress flag package help, we need to set Usage to empty func?

			// Actually fs.Parse prints usage to stderr by default.
			// We can override Usage.
			return nil
		}
		return err
	}

	// Check required flags
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	for _, group := range requiredFlags {
		if group.fromEnv {
			continue
		}
		satisfied := false
		for _, name := range group.names {
			if visited[name] {
				satisfied = true
				break
			}
		}
		if !satisfied {
			return fmt.Errorf("missing required flag: %s", group.display)
		}
	}

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
						fieldVal := inst.Field(i)
						if fieldVal.Kind() == reflect.Func {
							sub.Func = fieldVal
						} else if sub.Value.CanAddr() {
							fieldVal.Set(sub.Value.Addr())
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
	var missingPositionals []string
	for i := 0; i < n.Type.NumField(); i++ {
		field := n.Type.Field(i)
		tag := field.Tag.Get("commander")
		if strings.Contains(tag, "positional") {
			required := false
			if tag != "" {
				parts := strings.Split(tag, ",")
				for _, p := range parts {
					if strings.TrimSpace(p) == "required" {
						required = true
						break
					}
				}
			}
			if posArgIdx >= len(remaining) {
				if required {
					missingPositionals = append(missingPositionals, field.Name)
				}
				continue
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
	if len(missingPositionals) > 0 {
		return fmt.Errorf("missing required positional: %s", missingPositionals[0])
	}

	// 5. Execute Run
	// We need to call Run on the Pointer to inst?
	// inst is addressable.
	method := inst.Addr().MethodByName("Run")
	if method.IsValid() {
		// Check for -h / --help in remaining args?
		// Actually Run handles args? No, Run is niladic.
		// If there are remaining args and Run is niladic, we might warn or error?
		// Unless they are positionals we already parsed.
		// Flags were parsed. Subcommands were checked. Positional args were consumed.
		// So `remaining` here should be empty if everything was consumed.
		// If not empty, user provided extra args.

		// Wait, `posArgIdx` tracks consumed positionals.
		// If `posArgIdx < len(remaining)`, we have unconsumed args.
		if posArgIdx < len(remaining) {
			// Check if help
			if remaining[posArgIdx] == "-h" || remaining[posArgIdx] == "--help" {
				printCommandHelp(n)
				return nil
			}
			return fmt.Errorf("unknown arguments: %v", remaining[posArgIdx:])
		}

		if method.Type().NumIn() == 0 {
			if method.Type().NumOut() == 0 {
				method.Call(nil)
				return nil
			}
			if method.Type().NumOut() == 1 && isErrorType(method.Type().Out(0)) {
				out := method.Call(nil)
				if len(out) == 1 {
					if err, ok := out[0].Interface().(error); ok && err != nil {
						return err
					}
				}
				return nil
			}
			return fmt.Errorf("command %s Run must return only error", n.Name)
		}
	}

	if len(n.Subcommands) > 0 {
		// Just list subcommands if we didn't run anything
		// Use the new help format
		printCommandHelp(n)
		return nil
	}

	return fmt.Errorf("command %s is not runnable (no Run method)", n.Name)
}

func printCommandHelp(node *CommandNode) {
	if node.Type == nil {
		fmt.Printf("Usage: %s\n\n", node.Name)
		if node.Description != "" {
			fmt.Println(node.Description)
		}
		return
	}

	fmt.Printf("Usage: %s [flags] [subcommand]\n\n", node.Name)

	// If description is empty, try to fetch it if we haven't already
	if node.Description == "" && node.RunMethod.IsValid() {
		// Wait, we only have RunMethod marker if parsing succeeded.
		// But getting doc requires the Run method to be available.
		// node.Value might be the struct value.
		// Let's try to get the method from Type or Value.
		// Actually parseStruct already calls getMethodDoc.
		// If it's empty, maybe it failed to parse the file.
	}

	if node.Description != "" {
		fmt.Println(node.Description)
		fmt.Println()
	}

	if len(node.Subcommands) > 0 {
		fmt.Println("Subcommands:")
		for name, sub := range node.Subcommands {
			fmt.Printf("  %-20s %s\n", name, sub.Description)
		}
		fmt.Println()
	}

	fmt.Println("Flags:")
	// Re-inspect flags to print help
	// We need to instantiate a flagset to use its PrintDefaults?
	// Or we can manually iterate fields like we do for completion/parsing.
	fs := flag.NewFlagSet(node.Name, flag.ContinueOnError)
	// We need to bind them to dummy vars to register them
	// This duplicates logic from execute.
	// For now, let's just use the same logic as execute to populate the flagset, then PrintDefaults.
	// But `inst` is not available here. We need a zero value.

	// inst := reflect.New(node.Type).Elem()
	for i := 0; i < node.Type.NumField(); i++ {
		field := node.Type.Field(i)
		tag := field.Tag.Get("commander")
		if strings.Contains(tag, "subcommand") {
			continue
		}

		name := strings.ToLower(field.Name)
		usage := ""
		shortName := ""

		if tag != "" {
			parts := strings.Split(tag, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "name=") {
					name = strings.TrimPrefix(p, "name=")
				} else if strings.HasPrefix(p, "short=") {
					shortName = strings.TrimPrefix(p, "short=")
				} else if strings.HasPrefix(p, "desc=") || strings.HasPrefix(p, "description=") {
					if strings.HasPrefix(p, "desc=") {
						usage = strings.TrimPrefix(p, "desc=")
					} else {
						usage = strings.TrimPrefix(p, "description=")
					}
				}
			}
		}

		switch field.Type.Kind() {
		case reflect.String:
			fs.StringVar(new(string), name, "", usage)
			if shortName != "" {
				fs.StringVar(new(string), shortName, "", usage)
			}
		case reflect.Int:
			fs.IntVar(new(int), name, 0, usage)
			if shortName != "" {
				fs.IntVar(new(int), shortName, 0, usage)
			}
		case reflect.Bool:
			fs.BoolVar(new(bool), name, false, usage)
			if shortName != "" {
				fs.BoolVar(new(bool), shortName, false, usage)
			}
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
}

// DetectRootCommands filters a list of possible command objects to find those
// that are NOT subcommands of any other command in the list.
// It uses the `commander:"subcommand"` tag to identify relationships.
func DetectRootCommands(candidates ...interface{}) []interface{} {
	// 1. Find all types that are referenced as subcommands
	subcommandTypes := make(map[reflect.Type]bool)

	for _, c := range candidates {
		v := reflect.ValueOf(c)
		t := v.Type()
		// Handle pointer to struct
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			continue
		}

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			tag := field.Tag.Get("commander")
			if strings.Contains(tag, "subcommand") {
				// This field type is a subcommand
				subType := field.Type
				if subType.Kind() == reflect.Ptr {
					subType = subType.Elem()
				}
				subcommandTypes[subType] = true
			}
		}
	}

	// 2. Filter candidates
	var roots []interface{}
	for _, c := range candidates {
		v := reflect.ValueOf(c)
		t := v.Type()
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}

		if !subcommandTypes[t] {
			roots = append(roots, c)
		}
	}

	return roots
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
