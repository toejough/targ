package targs

import (
	"context"
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

	printTargsOptions()
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
	Parent      *CommandNode
	Subcommands map[string]*CommandNode
	RunMethod   reflect.Value
	Description string
}

type flagSpec struct {
	value          reflect.Value
	name           string
	short          string
	usage          string
	env            string
	defaultValue   *string
	required       bool
	defaultApplied bool
	envApplied     bool
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
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("targs")
		if tag == "" {
			continue
		}
		fieldVal := v.Field(i)
		if strings.Contains(tag, "subcommand") {
			if fieldVal.Kind() == reflect.Func {
				continue
			}
			if !fieldVal.IsZero() {
				return nil, fmt.Errorf("command %s must not prefill subcommand %s; use default tags instead", typ.Name(), field.Name)
			}
			continue
		}
		if !fieldVal.IsZero() {
			return nil, fmt.Errorf("command %s must be zero value; use default tags instead of prefilled fields", typ.Name())
		}
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
	if cmdName := getCommandName(v, typ); cmdName != "" {
		node.Name = cmdName
	}
	if desc := getDescription(v, typ); desc != "" {
		node.Description = desc
	}

	// 2. Look for fields with `targs:"subcommand"`
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("targs")
		if strings.Contains(tag, "subcommand") {
			// This field is a subcommand
			// Recurse

			fieldType := field.Type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			var subNode *CommandNode
			if fieldType.Kind() == reflect.Func {
				if err := validateFuncType(field.Type); err != nil {
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
			subNode.Parent = node

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

func (n *CommandNode) execute(ctx context.Context, args []string) error {
	return n.executeWithParents(ctx, args, nil, map[string]bool{})
}

type commandInstance struct {
	node  *CommandNode
	value reflect.Value
}

func (n *CommandNode) executeWithParents(
	ctx context.Context,
	args []string,
	parents []commandInstance,
	visited map[string]bool,
) error {
	if n.Func.IsValid() {
		return executeFunctionWithParents(ctx, args, n, parents, visited)
	}

	inst := nodeInstance(n)
	chain := append(parents, commandInstance{node: n, value: inst})

	fs := flag.NewFlagSet(n.Name, flag.ContinueOnError)
	fs.Usage = func() { printCommandHelp(n) }

	specs, longNames, err := registerChainFlags(fs, chain)
	if err != nil {
		return err
	}
	expandedArgs, err := expandShortFlagGroups(args, specs)
	if err != nil {
		return err
	}
	if err := validateLongFlagArgs(expandedArgs, longNames); err != nil {
		return err
	}
	if err := fs.Parse(expandedArgs); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})

	remaining := fs.Args()
	if len(remaining) > 0 {
		subName := remaining[0]
		if sub, ok := n.Subcommands[subName]; ok {
			if err := assignSubcommandField(n, inst, subName, sub); err != nil {
				return err
			}
			return sub.executeWithParents(ctx, remaining[1:], chain, visited)
		}
	}

	if err := applyDefaultsAndEnv(specs, visited); err != nil {
		return err
	}
	if err := checkRequiredFlags(specs, visited); err != nil {
		return err
	}
	posArgIdx, err := applyPositionals(inst, n, remaining)
	if err != nil {
		return err
	}

	if err := runPersistentHooks(ctx, chain, "PersistentBefore"); err != nil {
		return err
	}
	if err := runCommand(ctx, n, inst, remaining, posArgIdx); err != nil {
		return err
	}
	if err := runPersistentHooks(ctx, reverseChain(chain), "PersistentAfter"); err != nil {
		return err
	}
	return nil
}

func executeFunctionWithParents(
	ctx context.Context,
	args []string,
	node *CommandNode,
	parents []commandInstance,
	visited map[string]bool,
) error {
	fs := flag.NewFlagSet(node.Name, flag.ContinueOnError)
	fs.Usage = func() { printCommandHelp(node) }

	specs, longNames, err := registerChainFlags(fs, parents)
	if err != nil {
		return err
	}
	expandedArgs, err := expandShortFlagGroups(args, specs)
	if err != nil {
		return err
	}
	if err := validateLongFlagArgs(expandedArgs, longNames); err != nil {
		return err
	}
	if err := fs.Parse(expandedArgs); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})

	remaining := fs.Args()
	if err := applyDefaultsAndEnv(specs, visited); err != nil {
		return err
	}
	if err := checkRequiredFlags(specs, visited); err != nil {
		return err
	}
	if len(remaining) > 0 {
		return fmt.Errorf("unexpected argument: %s", remaining[0])
	}
	if err := runPersistentHooks(ctx, parents, "PersistentBefore"); err != nil {
		return err
	}
	if err := callFunction(ctx, node.Func); err != nil {
		return err
	}
	if err := runPersistentHooks(ctx, reverseChain(parents), "PersistentAfter"); err != nil {
		return err
	}
	return nil
}

func nodeInstance(node *CommandNode) reflect.Value {
	if node != nil && node.Value.IsValid() && node.Value.Kind() == reflect.Struct && node.Value.CanAddr() {
		return node.Value
	}
	if node != nil && node.Type != nil {
		inst := reflect.New(node.Type).Elem()
		if node.Value.IsValid() && node.Value.Kind() == reflect.Struct {
			typ := node.Type
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				tag := field.Tag.Get("targs")
				if strings.Contains(tag, "subcommand") && field.Type.Kind() == reflect.Func {
					inst.Field(i).Set(node.Value.Field(i))
				}
			}
		}
		return inst
	}
	return reflect.Value{}
}

type dynamicFlagValue struct {
	set    func(string) error
	str    func() string
	isBool bool
}

func (d *dynamicFlagValue) String() string {
	if d.str == nil {
		return ""
	}
	return d.str()
}

func (d *dynamicFlagValue) Set(value string) error {
	if d.set == nil {
		return fmt.Errorf("no setter defined")
	}
	return d.set(value)
}

func (d *dynamicFlagValue) IsBoolFlag() bool {
	return d.isBool
}

func registerChainFlags(fs *flag.FlagSet, chain []commandInstance) ([]*flagSpec, map[string]bool, error) {
	var specs []*flagSpec
	longNames := map[string]bool{}
	usedNames := map[string]bool{}

	for _, inst := range chain {
		if inst.node == nil || inst.node.Type == nil {
			continue
		}
		typ := inst.node.Type
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			fieldVal := inst.value.Field(i)
			spec, ok, err := flagSpecForField(field, fieldVal)
			if err != nil {
				return nil, nil, err
			}
			if !ok {
				continue
			}
			if usedNames[spec.name] {
				return nil, nil, fmt.Errorf("flag %s already defined", spec.name)
			}
			usedNames[spec.name] = true
			if spec.short != "" {
				if usedNames[spec.short] {
					return nil, nil, fmt.Errorf("flag %s already defined", spec.short)
				}
				usedNames[spec.short] = true
			}
			longNames[spec.name] = true
			specs = append(specs, spec)

			specLocal := spec
			flagValue := &dynamicFlagValue{
				set: func(value string) error {
					return setFieldFromString(specLocal.value, value)
				},
				str: func() string {
					return fmt.Sprint(specLocal.value.Interface())
				},
				isBool: specLocal.value.Kind() == reflect.Bool,
			}
			fs.Var(flagValue, spec.name, spec.usage)
			if spec.short != "" {
				fs.Var(flagValue, spec.short, spec.usage)
			}
		}
	}
	return specs, longNames, nil
}

func flagSpecForField(field reflect.StructField, fieldVal reflect.Value) (*flagSpec, bool, error) {
	tag := field.Tag.Get("targs")
	if !field.IsExported() {
		if strings.TrimSpace(tag) != "" {
			return nil, false, fmt.Errorf("field %s must be exported", field.Name)
		}
		return nil, false, nil
	}
	if strings.Contains(tag, "subcommand") || strings.Contains(tag, "positional") {
		return nil, false, nil
	}

	name := strings.ToLower(field.Name)
	shortName := ""
	usage := ""
	env := ""
	required := false
	var defaultValue *string

	if tag != "" {
		parts := strings.Split(tag, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			switch {
			case strings.HasPrefix(p, "name="):
				name = strings.TrimPrefix(p, "name=")
			case strings.HasPrefix(p, "short="):
				shortName = strings.TrimPrefix(p, "short=")
			case strings.HasPrefix(p, "env="):
				env = strings.TrimPrefix(p, "env=")
			case strings.HasPrefix(p, "default="):
				val := strings.TrimPrefix(p, "default=")
				defaultValue = &val
			case strings.HasPrefix(p, "desc="):
				usage = strings.TrimPrefix(p, "desc=")
			case strings.HasPrefix(p, "description="):
				usage = strings.TrimPrefix(p, "description=")
			case p == "required":
				required = true
			}
		}
	}

	return &flagSpec{
		value:        fieldVal,
		name:         name,
		short:        shortName,
		usage:        usage,
		env:          env,
		defaultValue: defaultValue,
		required:     required,
	}, true, nil
}

func applyDefaultsAndEnv(specs []*flagSpec, visited map[string]bool) error {
	for _, spec := range specs {
		if flagVisited(spec, visited) {
			continue
		}
		if spec.env != "" {
			if value := os.Getenv(spec.env); value != "" {
				if err := setFieldFromString(spec.value, value); err != nil {
					return fmt.Errorf("invalid value for env %s: %w", spec.env, err)
				}
				spec.envApplied = true
				continue
			}
		}
		if spec.defaultValue != nil {
			if err := setFieldFromString(spec.value, *spec.defaultValue); err != nil {
				return fmt.Errorf("invalid default for --%s: %w", spec.name, err)
			}
			spec.defaultApplied = true
		}
	}
	return nil
}

func checkRequiredFlags(specs []*flagSpec, visited map[string]bool) error {
	for _, spec := range specs {
		if !spec.required {
			continue
		}
		if flagVisited(spec, visited) || spec.defaultApplied || spec.envApplied {
			continue
		}
		display := fmt.Sprintf("--%s", spec.name)
		if spec.short != "" {
			display = fmt.Sprintf("--%s, -%s", spec.name, spec.short)
		}
		return fmt.Errorf("missing required flag %s", display)
	}
	return nil
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

func applyPositionals(inst reflect.Value, node *CommandNode, args []string) (int, error) {
	if node == nil || node.Type == nil {
		if len(args) > 0 {
			return 0, fmt.Errorf("unexpected argument: %s", args[0])
		}
		return 0, nil
	}
	typ := node.Type
	posIndex := 0
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("targs")
		if !strings.Contains(tag, "positional") {
			continue
		}
		if !field.IsExported() {
			if strings.TrimSpace(tag) != "" {
				return posIndex, fmt.Errorf("field %s must be exported", field.Name)
			}
			continue
		}
		fieldVal := inst.Field(i)
		required := false
		displayName := field.Name
		var defaultValue *string

		parts := strings.Split(tag, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			switch {
			case strings.HasPrefix(p, "name="):
				displayName = strings.TrimPrefix(p, "name=")
			case strings.HasPrefix(p, "default="):
				val := strings.TrimPrefix(p, "default=")
				defaultValue = &val
			case p == "required":
				required = true
			}
		}

		if posIndex < len(args) {
			if err := setFieldFromString(fieldVal, args[posIndex]); err != nil {
				return posIndex, err
			}
			posIndex++
			continue
		}
		if defaultValue != nil {
			if err := setFieldFromString(fieldVal, *defaultValue); err != nil {
				return posIndex, err
			}
			continue
		}
		if required {
			return posIndex, fmt.Errorf("missing required positional %s", displayName)
		}
	}
	if posIndex < len(args) {
		return posIndex, fmt.Errorf("unexpected argument: %s", args[posIndex])
	}
	return posIndex, nil
}

func assignSubcommandField(parent *CommandNode, parentInst reflect.Value, subName string, sub *CommandNode) error {
	if parent == nil || parent.Type == nil {
		return nil
	}
	typ := parent.Type
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("targs")
		if !strings.Contains(tag, "subcommand") {
			continue
		}
		name := camelToKebab(field.Name)
		if tag != "" {
			parts := strings.Split(tag, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "name=") {
					name = strings.TrimPrefix(p, "name=")
				} else if strings.HasPrefix(p, "subcommand=") {
					name = strings.TrimPrefix(p, "subcommand=")
				}
			}
		}
		if name != subName {
			continue
		}

		fieldVal := parentInst.Field(i)
		fieldType := field.Type
		if fieldType.Kind() == reflect.Func {
			if fieldVal.IsNil() {
				return fmt.Errorf("subcommand %s is nil", subName)
			}
			sub.Func = fieldVal
			return nil
		}

		if fieldType.Kind() == reflect.Ptr {
			newInst := reflect.New(fieldType.Elem())
			fieldVal.Set(newInst)
			sub.Value = newInst.Elem()
			return nil
		}

		if fieldType.Kind() == reflect.Struct {
			newInst := reflect.New(fieldType).Elem()
			fieldVal.Set(newInst)
			sub.Value = newInst
			return nil
		}
	}
	return nil
}

func runPersistentHooks(ctx context.Context, chain []commandInstance, methodName string) error {
	for _, inst := range chain {
		if inst.node == nil || inst.node.Type == nil {
			continue
		}
		if _, err := callMethod(ctx, inst.value, methodName); err != nil {
			return err
		}
	}
	return nil
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

func runCommand(ctx context.Context, node *CommandNode, inst reflect.Value, args []string, posArgIdx int) error {
	if node == nil {
		return nil
	}
	_, err := callMethod(ctx, inst, "Run")
	return err
}

func callFunction(ctx context.Context, fn reflect.Value) error {
	if !fn.IsValid() || (fn.Kind() == reflect.Func && fn.IsNil()) {
		return fmt.Errorf("nil function command")
	}
	if err := validateFuncType(fn.Type()); err != nil {
		return err
	}
	var args []reflect.Value
	if fn.Type().NumIn() == 1 {
		args = []reflect.Value{reflect.ValueOf(ctx)}
	}
	results := fn.Call(args)
	if len(results) == 1 && !results[0].IsNil() {
		return results[0].Interface().(error)
	}
	return nil
}

func callMethod(ctx context.Context, receiver reflect.Value, name string) (bool, error) {
	if !receiver.IsValid() {
		return false, nil
	}
	target := receiver
	if receiver.Kind() != reflect.Ptr {
		if receiver.CanAddr() {
			target = receiver.Addr()
		}
	}
	method := target.MethodByName(name)
	if !method.IsValid() {
		return false, nil
	}
	mtype := method.Type()
	if mtype.NumIn() > 1 {
		return true, fmt.Errorf("%s must accept context.Context or no args", name)
	}
	var callArgs []reflect.Value
	if mtype.NumIn() == 1 {
		if !isContextType(mtype.In(0)) {
			return true, fmt.Errorf("%s must accept context.Context", name)
		}
		callArgs = []reflect.Value{reflect.ValueOf(ctx)}
	}
	if mtype.NumOut() > 1 {
		return true, fmt.Errorf("%s must return only error", name)
	}
	if mtype.NumOut() == 1 && !isErrorType(mtype.Out(0)) {
		return true, fmt.Errorf("%s must return only error", name)
	}
	results := method.Call(callArgs)
	if len(results) == 1 && !results[0].IsNil() {
		return true, results[0].Interface().(error)
	}
	return true, nil
}

func printCommandHelp(node *CommandNode) {
	if node.Type == nil {
		fmt.Printf("Usage: %s\n\n", node.Name)
		if node.Description != "" {
			fmt.Println(node.Description)
		}
		return
	}

	usageLine := buildUsageLine(node)
	fmt.Printf("Usage: %s\n\n", usageLine)

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

	flags := collectFlagHelpChain(node)
	if len(flags) > 0 {
		fmt.Println("Flags:")
		for _, item := range flags {
			name := fmt.Sprintf("--%s", item.Name)
			if item.Short != "" {
				name = fmt.Sprintf("--%s, -%s", item.Name, item.Short)
			}
			if item.Placeholder != "" && item.Placeholder != "[flag]" {
				name = fmt.Sprintf("%s %s", name, item.Placeholder)
			}
			usage := item.Usage
			if item.Options != "" {
				if usage == "" {
					usage = fmt.Sprintf("options: %s", item.Options)
				} else {
					usage = fmt.Sprintf("%s (options: %s)", usage, item.Options)
				}
			}
			fmt.Printf("  %-24s %s\n", name, usage)
		}
	}
}

func printTargsOptions() {
	fmt.Println("\nTargs options:")
	fmt.Println("  --help")
	fmt.Println("  --completion [bash|zsh|fish]")
}

func buildUsageLine(node *CommandNode) string {
	parts := []string{node.Name}
	flags := collectFlagHelpChain(node)
	for _, item := range flags {
		if item.Required {
			parts = append(parts, formatFlagUsage(item))
		} else {
			parts = append(parts, fmt.Sprintf("[%s]", formatFlagUsage(item)))
		}
	}
	if len(node.Subcommands) > 0 {
		parts = append(parts, "[subcommand]")
	}
	positionals := collectPositionalHelp(node)
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
			parts = append(parts, fmt.Sprintf("[%s]", name))
		}
	}
	return strings.Join(parts, " ")
}

func formatFlagUsage(item flagHelp) string {
	name := fmt.Sprintf("--%s", item.Name)
	if item.Short != "" {
		name = fmt.Sprintf("{-%s|--%s}", item.Short, item.Name)
	}
	if item.Placeholder != "" && item.Placeholder != "[flag]" {
		name = fmt.Sprintf("%s %s", name, item.Placeholder)
	}
	return name
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

func collectFlagHelp(node *CommandNode) []flagHelp {
	if node.Type == nil {
		return nil
	}
	typ := node.Type
	var flags []flagHelp
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("targs")
		if strings.Contains(tag, "subcommand") || strings.Contains(tag, "positional") {
			continue
		}
		if !field.IsExported() {
			continue
		}

		name := strings.ToLower(field.Name)
		shortName := ""
		usage := ""
		options := ""
		placeholder := ""
		required := false

		if tag != "" {
			parts := strings.Split(tag, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "name=") {
					name = strings.TrimPrefix(p, "name=")
				} else if strings.HasPrefix(p, "short=") {
					shortName = strings.TrimPrefix(p, "short=")
				} else if strings.HasPrefix(p, "enum=") {
					options = strings.TrimPrefix(p, "enum=")
				} else if strings.HasPrefix(p, "placeholder=") {
					placeholder = strings.TrimPrefix(p, "placeholder=")
				} else if strings.HasPrefix(p, "desc=") || strings.HasPrefix(p, "description=") {
					if strings.HasPrefix(p, "desc=") {
						usage = strings.TrimPrefix(p, "desc=")
					} else {
						usage = strings.TrimPrefix(p, "description=")
					}
				} else if p == "required" {
					required = true
				}
			}
		}

		if options != "" {
			placeholder = fmt.Sprintf("{%s}", options)
			options = ""
		}
		if placeholder == "" {
			switch field.Type.Kind() {
			case reflect.String:
				placeholder = "<string>"
			case reflect.Int:
				placeholder = "<int>"
			case reflect.Bool:
				placeholder = "[flag]"
			}
		}

		flags = append(flags, flagHelp{
			Name:        name,
			Short:       shortName,
			Usage:       usage,
			Options:     options,
			Placeholder: placeholder,
			Required:    required,
		})
	}
	return flags
}

func collectFlagHelpChain(node *CommandNode) []flagHelp {
	chain := nodeChain(node)
	var flags []flagHelp
	for i, current := range chain {
		inherited := i < len(chain)-1
		for _, item := range collectFlagHelp(current) {
			item.Inherited = inherited
			flags = append(flags, item)
		}
	}
	return flags
}

type positionalHelp struct {
	Name        string
	Placeholder string
	Required    bool
}

func collectPositionalHelp(node *CommandNode) []positionalHelp {
	if node.Type == nil {
		return nil
	}
	typ := node.Type
	var positionals []positionalHelp
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("targs")
		if !strings.Contains(tag, "positional") {
			continue
		}
		name := strings.ToUpper(field.Name)
		placeholder := ""
		options := ""
		required := false
		if tag != "" {
			parts := strings.Split(tag, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "name=") {
					name = strings.ToUpper(strings.TrimPrefix(p, "name="))
				} else if strings.HasPrefix(p, "enum=") {
					options = strings.TrimPrefix(p, "enum=")
				} else if strings.HasPrefix(p, "placeholder=") {
					placeholder = strings.TrimPrefix(p, "placeholder=")
				} else if p == "required" {
					required = true
				}
			}
		}
		if options != "" {
			placeholder = fmt.Sprintf("{%s}", options)
		}
		positionals = append(positionals, positionalHelp{
			Name:        name,
			Placeholder: placeholder,
			Required:    required,
		})
	}
	return positionals
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
			return fmt.Errorf("long flags must use --%s (got -%s)", name, name)
		}
	}
	return nil
}

func expandShortFlagGroups(args []string, specs []*flagSpec) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}
	shortInfo := map[string]bool{}
	longInfo := map[string]bool{}
	for _, spec := range specs {
		longInfo[spec.name] = true
		if spec.short == "" {
			continue
		}
		shortInfo[spec.short] = spec.value.Kind() == reflect.Bool
	}
	var expanded []string
	for _, arg := range args {
		if arg == "--" {
			expanded = append(expanded, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") || len(arg) <= 2 || !strings.HasPrefix(arg, "-") {
			expanded = append(expanded, arg)
			continue
		}
		if strings.Contains(arg, "=") {
			expanded = append(expanded, arg)
			continue
		}
		group := strings.TrimPrefix(arg, "-")
		if len(group) <= 1 {
			expanded = append(expanded, arg)
			continue
		}
		if longInfo[group] {
			expanded = append(expanded, arg)
			continue
		}
		allBool := true
		unknown := false
		for _, ch := range group {
			name := string(ch)
			isBool, ok := shortInfo[name]
			if !ok {
				unknown = true
				allBool = false
				break
			}
			if !isBool {
				allBool = false
				break
			}
		}
		if unknown {
			expanded = append(expanded, arg)
			continue
		}
		if !allBool {
			return nil, fmt.Errorf("short flag group %q must contain only boolean flags", arg)
		}
		for _, ch := range group {
			expanded = append(expanded, "-"+string(ch))
		}
	}
	return expanded, nil
}

func nodeChain(node *CommandNode) []*CommandNode {
	if node == nil {
		return nil
	}
	var chain []*CommandNode
	for current := node; current != nil; current = current.Parent {
		chain = append(chain, current)
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// DetectRootCommands filters a list of possible command objects to find those
// that are NOT subcommands of any other command in the list.
// It uses the `targs:"subcommand"` tag to identify relationships.
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
			tag := field.Tag.Get("targs")
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
