package commander

import (
	"flag"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"os"
	"unicode"
)

// Run executes the CLI. It accepts a list of functions or structs with methods.
func Run(targets ...interface{}) {
	commands := []*commandDefinition{}

	for _, t := range targets {
		cmds, err := parseTarget(t)
		if err != nil {
			fmt.Printf("Error parsing target: %v\n", err)
			continue
		}
		commands = append(commands, cmds...)
	}

	if len(os.Args) < 2 {
		printUsage(commands)
		return
	}

	args := os.Args[1:]
	
	// Check for global help
	if args[0] == "-h" || args[0] == "--help" {
		printUsage(commands)
		return
	}

	// Try to find the longest matching command
	var matchedCmd *commandDefinition
	var matchedLen int

	for _, cmd := range commands {
		cmdParts := strings.Fields(cmd.Name)
		if len(args) >= len(cmdParts) {
			match := true
			for i, part := range cmdParts {
				if !strings.EqualFold(args[i], part) {
					match = false
					break
				}
			}
			if match {
				if len(cmdParts) > matchedLen {
					matchedCmd = cmd
					matchedLen = len(cmdParts)
				}
			}
		}
	}

	if matchedCmd != nil {
		// Found a command
		remainingArgs := args[matchedLen:]
		
		// If help flag is present in remaining args, show help for this command
		for _, arg := range remainingArgs {
			if arg == "-h" || arg == "--help" {
				fmt.Printf("Usage: %s [flags]\n", matchedCmd.Name)
				// TODO: Show flags help
				return
			}
		}
		
		if err := matchedCmd.execute(remainingArgs); err != nil {
			fmt.Printf("Error executing command: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Printf("Unknown command: %s\n", strings.Join(args, " "))
	printUsage(commands)
	os.Exit(1)
}

func printUsage(cmds []*commandDefinition) {
	fmt.Println("Usage: <command> [args]")
	fmt.Println("\nAvailable commands:")
	for _, cmd := range cmds {
		fmt.Printf("  %s\n", cmd.Name)
	}
}

type commandDefinition struct {
	Name string
	Func reflect.Value
	Args reflect.Type
	IsRoot bool
	Receiver reflect.Value
}

// parseTarget parses a struct into one or more command definitions
func parseTarget(t interface{}) ([]*commandDefinition, error) {
	v := reflect.ValueOf(t)
	typ := v.Type()

	if typ.Kind() == reflect.Func {
		return nil, fmt.Errorf("functions are not supported, use a struct with a Run method")
	}
	
	if typ.Kind() != reflect.Struct && (typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct) {
		return nil, fmt.Errorf("unsupported type: %v", typ.Kind())
	}

	var cmds []*commandDefinition
	
	// Determine struct name for prefix
	var structName string
	if typ.Kind() == reflect.Ptr {
		structName = typ.Elem().Name()
	} else {
		structName = typ.Name()
	}
	prefix := camelToKebab(structName)

	// Check for Run method (Command on the struct itself)
	// We check the signature:
	// - Run() -> Root command (execute the struct)
	// - Run(args) -> Subcommand named "run"
	
	runMethod := v.MethodByName("Run")
	handledRunAsRoot := false
	
	if runMethod.IsValid() && runMethod.Type().NumIn() == 0 {
		// Found niladic Run method -> It is the Root command.
		// The arguments are the fields of the struct itself.
		
		// We need the Type of the struct to parse flags into.
		argType := typ
		if typ.Kind() == reflect.Ptr {
			argType = typ.Elem()
		}
		
		cmd := &commandDefinition{
			Name: prefix,
			Func: runMethod,
			Args: argType,
			IsRoot: true, // Marker to indicate arguments are on the receiver, not passed to func
			Receiver: v,
		}
		cmds = append(cmds, cmd)
		handledRunAsRoot = true
	}

	// Check for other methods (Subcommands)
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if method.PkgPath != "" {
			continue // unexported
		}
		if method.Name == "Run" && handledRunAsRoot {
			continue // handled as root
		}
		
		boundMethod := v.Method(i)
		cmd, err := parseFuncValue(boundMethod, prefix+" "+camelToKebab(method.Name))
		if err != nil {
			continue 
		}
		cmds = append(cmds, cmd)
	}

	return cmds, nil
}

func parseFunc(f interface{}, nameOverride string) (*commandDefinition, error) {
	return parseFuncValue(reflect.ValueOf(f), nameOverride)
}

func parseFuncValue(v reflect.Value, nameOverride string) (*commandDefinition, error) {
	t := v.Type()
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("expected function, got %v", t.Kind())
	}

	if t.NumIn() != 1 {
		return nil, fmt.Errorf("function must take exactly one argument")
	}

	argType := t.In(0)
	if argType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("argument must be a struct")
	}

	name := nameOverride
	if name == "" {
		name = getFunctionNameFromValue(v)
		name = camelToKebab(name)
	}
	
	return &commandDefinition{
		Name: name,
		Func: v,
		Args: argType,
	}, nil
}

func getFunctionNameFromValue(v reflect.Value) string {
	ptr := v.Pointer()
	fn := runtime.FuncForPC(ptr)
	if fn == nil {
		return "unknown"
	}
	full := fn.Name()
	parts := strings.Split(full, ".")
	return parts[len(parts)-1]
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

func (c *commandDefinition) execute(args []string) error {
	var argStruct reflect.Value
	
	if c.IsRoot {
		// Ensure receiver is modifiable
		if c.Receiver.Kind() == reflect.Ptr {
			argStruct = c.Receiver.Elem()
		} else {
			// If value, we can't modify it easily for the bound method
			// But since we are creating a new CLI run, we can create a new instance
			// However, c.Func is already bound to the original value.
			
			// Let's create a new instance of the args struct to parse flags into
			argStructPtr := reflect.New(c.Args)
			argStruct = argStructPtr.Elem()
			
			// Note: We won't be able to apply this to the bound receiver if it's a value.
			// Users should use pointer receivers for `Run` if they have flags.
			// But we'll parse anyway.
		}
	} else {
		argStructPtr := reflect.New(c.Args)
		argStruct = argStructPtr.Elem()
	}

	fs := flag.NewFlagSet(c.Name, flag.ContinueOnError)
	
	requiredFields := []string{}
	positionalFields := []int{}
	envValues := make(map[string]bool)

	for i := 0; i < c.Args.NumField(); i++ {
		field := c.Args.Field(i)
		fieldName := strings.ToLower(field.Name)
		usage := ""
		required := false
		defaultValue := ""
		isPositional := false
		
		tag := field.Tag.Get("commander")
		if tag != "" {
			parts := strings.Split(tag, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "name=") {
					fieldName = strings.TrimPrefix(part, "name=")
				} else if strings.HasPrefix(part, "desc=") || strings.HasPrefix(part, "description=") {
					if strings.HasPrefix(part, "desc=") {
						usage = strings.TrimPrefix(part, "desc=")
					} else {
						usage = strings.TrimPrefix(part, "description=")
					}
				} else if part == "required" {
					required = true
				} else if strings.HasPrefix(part, "env=") {
					envVar := strings.TrimPrefix(part, "env=")
					if val, ok := os.LookupEnv(envVar); ok {
						defaultValue = val
						envValues[fieldName] = true
					}
				} else if part == "positional" {
					isPositional = true
				}
			}
		}
		
		if isPositional {
			positionalFields = append(positionalFields, i)
			continue
		}
		
		if required {
			requiredFields = append(requiredFields, fieldName)
			usage = usage + " (required)"
		}
		
		switch field.Type.Kind() {
		case reflect.String:
			fs.StringVar(argStruct.Field(i).Addr().Interface().(*string), fieldName, defaultValue, usage)
		case reflect.Int:
			intVal := 0
			if defaultValue != "" {
				fmt.Sscanf(defaultValue, "%d", &intVal)
			}
			fs.IntVar(argStruct.Field(i).Addr().Interface().(*int), fieldName, intVal, usage)
		case reflect.Bool:
			boolVal := false
			if defaultValue == "true" || defaultValue == "1" {
				boolVal = true
			}
			fs.BoolVar(argStruct.Field(i).Addr().Interface().(*bool), fieldName, boolVal, usage)
		}
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Handle positional arguments
	positionalArgs := fs.Args()
	argIdx := 0
	
	for _, fieldIndex := range positionalFields {
		if argIdx >= len(positionalArgs) {
			break
		}
		
		field := argStruct.Field(fieldIndex)
		
		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String {
			// Variadic: consume all remaining args
			remaining := positionalArgs[argIdx:]
			field.Set(reflect.ValueOf(remaining))
			argIdx = len(positionalArgs)
			break
		}
		
		val := positionalArgs[argIdx]
		
		switch field.Kind() {
		case reflect.String:
			field.SetString(val)
		case reflect.Int:
			var intVal int64
			fmt.Sscanf(val, "%d", &intVal)
			field.SetInt(intVal)
		}
		argIdx++
	}

	seen := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		seen[f.Name] = true
	})
	
	for _, req := range requiredFields {
		if !seen[req] && !envValues[req] {
			return fmt.Errorf("required flag -%s is missing", req)
		}
	}

	// For Root commands, the method is already bound to the receiver.
	// But if we populated a NEW struct (because receiver was value),
	// the bound method won't see the changes.
	
	// For Subcommands, c.Func takes the args struct as argument.
	if c.IsRoot {
		// If we had to create a new struct because receiver was not ptr,
		// we can't really execute properly.
		// But if receiver was ptr, we modified it in place via argStruct.
		c.Func.Call(nil) 
	} else {
		values := []reflect.Value{argStruct}
		c.Func.Call(values)
	}
	return nil
}
