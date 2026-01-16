// Package core provides the internal implementation of targ command parsing and execution.
package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// unexported constants.
const (
	flagPlaceholder = "[flag]"
)

// unexported variables.
var (
	tagPartSetters = []struct {
		prefix string
		apply  func(opts *TagOptions, val string)
	}{
		{"name=", func(opts *TagOptions, val string) { opts.Name = val }},
		{"subcommand=", func(opts *TagOptions, val string) { opts.Name = val }},
		{"short=", func(opts *TagOptions, val string) { opts.Short = val }},
		{"env=", func(opts *TagOptions, val string) { opts.Env = val }},
		{"default=", func(opts *TagOptions, val string) { opts.Default = &val }},
		{"enum=", func(opts *TagOptions, val string) { opts.Enum = val }},
		{"placeholder=", func(opts *TagOptions, val string) { opts.Placeholder = val }},
		{"desc=", func(opts *TagOptions, val string) { opts.Desc = val }},
		{"description=", func(opts *TagOptions, val string) { opts.Desc = val }},
	}
)

type commandInstance struct {
	node  *commandNode
	value reflect.Value
}

type commandNode struct {
	Name        string
	Type        reflect.Type
	Value       reflect.Value // The struct instance
	Func        reflect.Value // Niladic function target
	Parent      *commandNode
	Subcommands map[string]*commandNode
	RunMethod   reflect.Value
	Description string
}

// --- Execution ---

func (n *commandNode) execute(ctx context.Context, args []string, opts RunOptions) error {
	_, err := n.executeWithParents(ctx, args, nil, map[string]bool{}, false, opts)
	return err
}

// executeSiblings handles implicit sibling resolution for remaining args.
func (n *commandNode) executeSiblings(
	ctx context.Context,
	inst reflect.Value,
	chain []commandInstance,
	remaining []string,
	visited map[string]bool,
	opts RunOptions,
) ([]string, error) {
	for len(remaining) > 0 {
		if remaining[0] == "^" {
			return remaining[1:], nil
		}

		sibling := n.findSibling(remaining[0])
		if sibling == nil {
			break
		}

		if err := assignSubcommandField(n, inst, sibling.Name, sibling); err != nil {
			return nil, err
		}

		var err error

		remaining, err = sibling.executeWithParents(ctx, remaining[1:], chain, visited, true, opts)
		if err != nil {
			return nil, err
		}
	}

	return remaining, nil
}

// executeStructCommand handles execution of struct-based commands.
func (n *commandNode) executeStructCommand(
	ctx context.Context,
	args []string,
	parents []commandInstance,
	visited map[string]bool,
	explicit bool,
	opts RunOptions,
) ([]string, error) {
	inst, err := nodeInstance(n)
	if err != nil {
		return nil, err
	}

	chain := slices.Concat(parents, []commandInstance{{node: n, value: inst}})

	specs, _, err := collectFlagSpecs(chain)
	if err != nil {
		return nil, err
	}

	result, err := parseCommandArgs(n, inst, chain, args, visited, explicit, true, false)
	if err != nil {
		return nil, err
	}

	if result.subcommand != nil {
		return n.executeSubcommand(ctx, inst, chain, result, visited, opts)
	}

	if opts.HelpOnly {
		return result.remaining, nil
	}

	return n.runCommandWithHooks(ctx, inst, chain, specs, visited, result.remaining)
}

// executeSubcommand handles subcommand dispatch and sibling resolution.
func (n *commandNode) executeSubcommand(
	ctx context.Context,
	inst reflect.Value,
	chain []commandInstance,
	result parseResult,
	visited map[string]bool,
	opts RunOptions,
) ([]string, error) {
	if err := assignSubcommandField(n, inst, result.subcommand.Name, result.subcommand); err != nil {
		return nil, err
	}

	remaining, err := result.subcommand.executeWithParents(
		ctx,
		result.remaining,
		chain,
		visited,
		true,
		opts,
	)
	if err != nil {
		return nil, err
	}

	return n.executeSiblings(ctx, inst, chain, remaining, visited, opts)
}

func (n *commandNode) executeWithParents(
	ctx context.Context,
	args []string,
	parents []commandInstance,
	visited map[string]bool,
	explicit bool,
	opts RunOptions,
) ([]string, error) {
	if opts.HelpOnly {
		printCommandHelp(n)
		printTargOptions(opts)
		fmt.Println()
	}

	if remaining, done := handleHelpFlag(n, args, opts); done {
		return remaining, nil
	}

	ctx, args, cancel, err := applyTimeout(ctx, args, opts)
	if err != nil {
		return nil, err
	}

	if cancel != nil {
		defer cancel()
	}

	if n.Func.IsValid() {
		return executeFunctionWithParents(ctx, args, n, parents, visited, explicit, opts)
	}

	return n.executeStructCommand(ctx, args, parents, visited, explicit, opts)
}

// findSibling finds a sibling subcommand by name (case-insensitive).
func (n *commandNode) findSibling(name string) *commandNode {
	for subName, sub := range n.Subcommands {
		if strings.EqualFold(subName, name) {
			return sub
		}
	}

	return nil
}

// runCommandWithHooks executes the command with before/after hooks.
func (n *commandNode) runCommandWithHooks(
	ctx context.Context,
	inst reflect.Value,
	chain []commandInstance,
	specs []*flagSpec,
	visited map[string]bool,
	remaining []string,
) ([]string, error) {
	err := applyDefaultsAndEnv(specs, visited)
	if err != nil {
		return nil, err
	}

	err = checkRequiredFlags(specs, visited)
	if err != nil {
		return nil, err
	}

	err = runPersistentHooks(ctx, chain, "PersistentBefore")
	if err != nil {
		return nil, err
	}

	err = runCommand(ctx, n, inst, nil, 0)
	if err != nil {
		return nil, err
	}

	err = runPersistentHooks(ctx, reverseChain(chain), "PersistentAfter")
	if err != nil {
		return nil, err
	}

	return remaining, nil
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

type flagSpec struct {
	value          reflect.Value
	name           string
	short          string
	env            string
	defaultValue   *string
	required       bool
	defaultApplied bool
	envApplied     bool
}

type positionalHelp struct {
	Name        string
	Placeholder string
	Required    bool
}

func applyDefaultsAndEnv(specs []*flagSpec, visited map[string]bool) error {
	for _, spec := range specs {
		if flagVisited(spec, visited) {
			continue
		}

		if spec.env != "" {
			if value := os.Getenv(spec.env); value != "" {
				err := setFieldFromString(spec.value, value)
				if err != nil {
					return fmt.Errorf("invalid value for env %s: %w", spec.env, err)
				}

				spec.envApplied = true

				continue
			}
		}

		if spec.defaultValue != nil {
			err := setFieldFromString(spec.value, *spec.defaultValue)
			if err != nil {
				return fmt.Errorf("invalid default for --%s: %w", spec.name, err)
			}

			spec.defaultApplied = true
		}
	}

	return nil
}

// applyNameAndDescription overrides node name and description from methods.
func applyNameAndDescription(node *commandNode, v reflect.Value, typ reflect.Type) {
	if cmdName := getCommandName(v, typ); cmdName != "" {
		node.Name = cmdName
	}

	if desc := getDescription(v, typ); desc != "" {
		node.Description = desc
	}
}

// applyRunMethodDoc extracts and applies Run method documentation.
func applyRunMethodDoc(node *commandNode, typ reflect.Type) {
	ptrType := reflect.PointerTo(typ)

	runMethod, hasRun := ptrType.MethodByName("Run")
	if !hasRun {
		return
	}

	node.RunMethod = reflect.Value{} // Marker

	if doc := getMethodDoc(runMethod); doc != "" {
		node.Description = strings.TrimSpace(doc)
	}
}

func applyTagOptionsOverride(
	inst reflect.Value,
	field reflect.StructField,
	opts TagOptions,
) (TagOptions, error) {
	method := tagOptionsMethod(inst)
	if !method.IsValid() {
		return opts, nil
	}

	err := validateTagOptionsSignature(method)
	if err != nil {
		return opts, err
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(field.Name),
		reflect.ValueOf(opts),
	})

	return extractTagOptionsResult(results, opts)
}

func applyTagPart(opts *TagOptions, p string) {
	for _, setter := range tagPartSetters {
		if after, ok := strings.CutPrefix(p, setter.prefix); ok {
			setter.apply(opts, after)
			return
		}
	}

	if p == "required" {
		opts.Required = true
	}
}

// applyTimeout extracts timeout from args and creates context with deadline.
func applyTimeout(
	ctx context.Context,
	args []string,
	opts RunOptions,
) (context.Context, []string, context.CancelFunc, error) {
	if opts.DisableTimeout {
		return ctx, args, nil, nil
	}

	timeout, remaining, err := extractTimeout(args)
	if err != nil {
		return ctx, args, nil, err
	}

	if timeout <= 0 {
		return ctx, remaining, nil, nil
	}

	newCtx, cancel := context.WithTimeout(ctx, timeout)

	return newCtx, remaining, cancel, nil
}

func assignSubcommandField(
	parent *commandNode,
	parentInst reflect.Value,
	subName string,
	sub *commandNode,
) error {
	if parent == nil || parent.Type == nil {
		return nil
	}

	typ := parent.Type
	for i := range typ.NumField() {
		field := typ.Field(i)

		opts, err := tagOptionsForField(parentInst, field)
		if err != nil {
			return err
		}

		if opts.Kind != TagKindSubcommand || opts.Name != subName {
			continue
		}

		return assignSubcommandValue(parentInst.Field(i), field.Type, sub, subName)
	}

	return nil
}

func assignSubcommandValue(
	fieldVal reflect.Value,
	fieldType reflect.Type,
	sub *commandNode,
	subName string,
) error {
	switch fieldType.Kind() {
	case reflect.Func:
		if fieldVal.IsNil() {
			return fmt.Errorf("subcommand %s is nil", subName)
		}

		sub.Func = fieldVal
	case reflect.Ptr:
		newInst := reflect.New(fieldType.Elem())
		fieldVal.Set(newInst)
		sub.Value = newInst.Elem()
	case reflect.Struct:
		newInst := reflect.New(fieldType).Elem()
		fieldVal.Set(newInst)
		sub.Value = newInst
	default:
		return fmt.Errorf("unsupported subcommand field type %s for %s", fieldType.Kind(), subName)
	}

	return nil
}

// --- Help output ---

func binaryName() string {
	if name := os.Getenv("TARG_BIN_NAME"); name != "" {
		return name
	}

	if len(os.Args) == 0 {
		return "targ"
	}

	name := os.Args[0]
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	if idx := strings.LastIndex(name, "\\"); idx != -1 {
		name = name[idx+1:]
	}

	return name
}

func buildFlagMaps(specs []*flagSpec) (shortInfo, longInfo map[string]bool) {
	shortInfo = map[string]bool{}
	longInfo = map[string]bool{}

	for _, spec := range specs {
		longInfo[spec.name] = true
		if spec.short != "" {
			shortInfo[spec.short] = spec.value.Kind() == reflect.Bool
		}
	}

	return shortInfo, longInfo
}

func buildUsageLine(node *commandNode) (string, error) {
	parts := []string{node.Name}

	flags, err := collectFlagHelpChain(node)
	if err != nil {
		return "", err
	}

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

	positionals, err := collectPositionalHelp(node)
	if err != nil {
		return "", err
	}

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

	return strings.Join(parts, " "), nil
}

func callFunction(ctx context.Context, fn reflect.Value) error {
	if !fn.IsValid() || (fn.Kind() == reflect.Func && fn.IsNil()) {
		return errors.New("nil function command")
	}

	err := validateFuncType(fn.Type())
	if err != nil {
		return err
	}

	var args []reflect.Value
	if fn.Type().NumIn() == 1 {
		args = []reflect.Value{reflect.ValueOf(ctx)}
	}

	results := fn.Call(args)
	if len(results) == 1 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return err
		}
	}

	return nil
}

func callMethod(ctx context.Context, receiver reflect.Value, name string) error {
	method, ok := lookupMethod(receiver, name)
	if !ok {
		return nil
	}

	callArgs, err := validateMethodInputs(ctx, method, name)
	if err != nil {
		return err
	}

	if err := validateMethodOutputs(method, name); err != nil {
		return err
	}

	return invokeMethod(method, callArgs)
}

func callStringMethod(v reflect.Value, typ reflect.Type, method string) string {
	m := methodValue(v, typ, method)
	if !m.IsValid() {
		return ""
	}

	if m.Type().NumIn() != 0 || m.Type().NumOut() != 1 ||
		m.Type().Out(0).Kind() != reflect.String {
		return ""
	}

	out := m.Call(nil)
	if len(out) == 0 {
		return ""
	}

	s, ok := out[0].Interface().(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(s)
}

// --- Utilities ---

func camelToKebab(s string) string {
	var result strings.Builder

	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			// Insert hyphen if previous is lowercase (e.g., fooBar -> foo-bar)
			// OR if we're at the start of a new word after an acronym (e.g., APIServer -> api-server)
			if unicode.IsLower(prev) || (i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
				result.WriteRune('-')
			}
		}

		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

func checkRequiredFlags(specs []*flagSpec, visited map[string]bool) error {
	for _, spec := range specs {
		if !spec.required {
			continue
		}

		if flagVisited(spec, visited) || spec.defaultApplied || spec.envApplied {
			continue
		}

		display := "--" + spec.name
		if spec.short != "" {
			display = fmt.Sprintf("--%s, -%s", spec.name, spec.short)
		}

		return fmt.Errorf("missing required flag %s", display)
	}

	return nil
}

func collectFlagHelp(node *commandNode) ([]flagHelp, error) {
	if node.Type == nil {
		return nil, nil
	}

	typ := node.Type
	inst := tagOptionsInstance(node)

	var flags []flagHelp

	for i := range typ.NumField() {
		field := typ.Field(i)

		help, ok, err := flagHelpForField(inst, field)
		if err != nil {
			return nil, err
		}

		if ok {
			flags = append(flags, help)
		}
	}

	return flags, nil
}

func collectFlagHelpChain(node *commandNode) ([]flagHelp, error) {
	chain := nodeChain(node)

	var flags []flagHelp

	for i, current := range chain {
		inherited := i < len(chain)-1

		items, err := collectFlagHelp(current)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			item.Inherited = inherited
			flags = append(flags, item)
		}
	}

	return flags, nil
}

func collectPositionalHelp(node *commandNode) ([]positionalHelp, error) {
	if node.Type == nil {
		return nil, nil
	}

	typ := node.Type
	inst := tagOptionsInstance(node)

	positionals := make([]positionalHelp, 0, typ.NumField())

	for i := range typ.NumField() {
		field := typ.Field(i)

		opts, err := tagOptionsForField(inst, field)
		if err != nil {
			return nil, err
		}

		if opts.Kind != TagKindPositional {
			continue
		}

		if !field.IsExported() {
			return nil, fmt.Errorf("field %s must be exported", field.Name)
		}

		placeholder := opts.Placeholder
		if opts.Enum != "" {
			placeholder = fmt.Sprintf("{%s}", opts.Enum)
		}

		positionals = append(positionals, positionalHelp{
			Name:        opts.Name,
			Placeholder: placeholder,
			Required:    opts.Required,
		})
	}

	return positionals, nil
}

func copySubcommandFuncs(inst reflect.Value, node *commandNode) error {
	if !node.Value.IsValid() || node.Value.Kind() != reflect.Struct {
		return nil
	}

	typ := node.Type
	for i := range typ.NumField() {
		field := typ.Field(i)

		opts, err := tagOptionsForField(node.Value, field)
		if err != nil {
			return err
		}

		if opts.Kind == TagKindSubcommand && field.Type.Kind() == reflect.Func {
			inst.Field(i).Set(node.Value.Field(i))
		}
	}

	return nil
}

// createCommandNode creates a new command node with default values.
func createCommandNode(v reflect.Value, typ reflect.Type) *commandNode {
	return &commandNode{
		Name:        camelToKebab(typ.Name()),
		Type:        typ,
		Value:       v,
		Subcommands: make(map[string]*commandNode),
	}
}

func detectTagKind(opts *TagOptions, tag, fieldName string) {
	if strings.Contains(tag, "subcommand") {
		opts.Kind = TagKindSubcommand
		opts.Name = camelToKebab(fieldName)
	}

	if strings.Contains(tag, "positional") {
		opts.Kind = TagKindPositional
		opts.Name = fieldName
	}
}

func executeFunctionWithParents(
	ctx context.Context,
	args []string,
	node *commandNode,
	parents []commandInstance,
	visited map[string]bool,
	explicit bool,
	opts RunOptions,
) ([]string, error) {
	specs, _, err := collectFlagSpecs(parents)
	if err != nil {
		return nil, err
	}

	result, err := parseCommandArgs(
		nil,
		reflect.Value{},
		parents,
		args,
		visited,
		explicit,
		true,
		false,
	)
	if err != nil {
		return nil, err
	}
	// In help-only mode, skip validation and execution
	if opts.HelpOnly {
		return result.remaining, nil
	}

	if err := applyDefaultsAndEnv(specs, visited); err != nil {
		return nil, err
	}

	if err := checkRequiredFlags(specs, visited); err != nil {
		return nil, err
	}

	if err := runPersistentHooks(ctx, parents, "PersistentBefore"); err != nil {
		return nil, err
	}

	if err := callFunction(ctx, node.Func); err != nil {
		return nil, err
	}

	if err := runPersistentHooks(ctx, reverseChain(parents), "PersistentAfter"); err != nil {
		return nil, err
	}

	return result.remaining, nil
}

func expandFlagGroup(arg, group string, shortInfo map[string]bool) ([]string, error) {
	allBool := true
	unknown := false

	for _, ch := range group {
		isBool, ok := shortInfo[string(ch)]
		if !ok {
			unknown = true

			break
		}

		if !isBool {
			allBool = false

			break
		}
	}

	if unknown {
		return []string{arg}, nil
	}

	if !allBool {
		return nil, fmt.Errorf("short flag group %q must contain only boolean flags", arg)
	}

	expanded := make([]string, 0, len(group))
	for _, ch := range group {
		expanded = append(expanded, "-"+string(ch))
	}

	return expanded, nil
}

func expandShortFlagGroups(args []string, specs []*flagSpec) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}

	shortInfo, longInfo := buildFlagMaps(specs)

	var expanded []string

	for _, arg := range args {
		if skipShortExpansion(arg, longInfo) {
			expanded = append(expanded, arg)
			continue
		}

		group := strings.TrimPrefix(arg, "-")

		expandedFlags, err := expandFlagGroup(arg, group, shortInfo)
		if err != nil {
			return nil, err
		}

		expanded = append(expanded, expandedFlags...)
	}

	return expanded, nil
}

func extractTagOptionsResult(results []reflect.Value, fallback TagOptions) (TagOptions, error) {
	if len(results) < 2 {
		return fallback, errors.New("TagOptions method returned wrong number of values")
	}

	if !results[1].IsNil() {
		err, ok := results[1].Interface().(error)
		if !ok {
			return fallback, errors.New("TagOptions method returned non-error type")
		}

		return fallback, err
	}

	tagOpts, ok := results[0].Interface().(TagOptions)
	if !ok {
		return fallback, errors.New("TagOptions method returned wrong type")
	}

	return tagOpts, nil
}

func flagHelpForField(inst reflect.Value, field reflect.StructField) (flagHelp, bool, error) {
	opts, err := tagOptionsForField(inst, field)
	if err != nil {
		return flagHelp{}, false, err
	}

	if opts.Kind != TagKindFlag {
		return flagHelp{}, false, nil
	}

	if !field.IsExported() {
		return flagHelp{}, false, fmt.Errorf("field %s must be exported", field.Name)
	}

	placeholder := resolvePlaceholder(opts, field.Type.Kind())

	return flagHelp{
		Name:        opts.Name,
		Short:       opts.Short,
		Usage:       opts.Desc,
		Options:     "",
		Placeholder: placeholder,
		Required:    opts.Required,
	}, true, nil
}

// --- Flag handling ---

func flagSpecForField(
	inst reflect.Value,
	field reflect.StructField,
	fieldVal reflect.Value,
) (*flagSpec, bool, error) {
	opts, err := tagOptionsForField(inst, field)
	if err != nil {
		return nil, false, err
	}

	if !field.IsExported() {
		return nil, false, fmt.Errorf("field %s must be exported", field.Name)
	}

	if opts.Kind != TagKindFlag {
		return nil, false, nil
	}

	return &flagSpec{
		value:        fieldVal,
		name:         opts.Name,
		short:        opts.Short,
		env:          opts.Env,
		defaultValue: opts.Default,
		required:     opts.Required,
	}, true, nil
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

func formatFlagName(item flagHelp) string {
	name := "--" + item.Name
	if item.Short != "" {
		name = fmt.Sprintf("--%s, -%s", item.Name, item.Short)
	}

	if item.Placeholder != "" && item.Placeholder != flagPlaceholder {
		name = fmt.Sprintf("%s %s", name, item.Placeholder)
	}

	return name
}

func formatFlagUsage(item flagHelp) string {
	name := "--" + item.Name
	if item.Short != "" {
		name = fmt.Sprintf("{-%s|--%s}", item.Short, item.Name)
	}

	if item.Placeholder != "" && item.Placeholder != flagPlaceholder {
		name = fmt.Sprintf("%s %s", name, item.Placeholder)
	}

	return name
}

func functionName(v reflect.Value) string {
	fn := runtime.FuncForPC(v.Pointer())
	if fn == nil {
		return ""
	}

	name := fn.Name()
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}

	name = strings.TrimSuffix(name, "-fm")

	return name
}

// --- Command metadata helpers ---

func getCommandName(v reflect.Value, typ reflect.Type) string {
	name := callStringMethod(v, typ, "Name")
	if name == "" {
		return ""
	}

	return camelToKebab(name)
}

func getDescription(v reflect.Value, typ reflect.Type) string {
	desc := callStringMethod(v, typ, "Description")
	return strings.TrimSpace(desc)
}

// handleHelpFlag checks for --help flag and prints help if requested.
func handleHelpFlag(n *commandNode, args []string, opts RunOptions) ([]string, bool) {
	if opts.DisableHelp || opts.HelpOnly {
		return nil, false
	}

	helpRequested, remaining := extractHelpFlag(args)
	if !helpRequested {
		return nil, false
	}

	printCommandHelp(n)
	printTargOptions(opts)

	return remaining, true
}

func invokeMethod(method reflect.Value, args []reflect.Value) error {
	results := method.Call(args)
	if len(results) == 1 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return err
		}
	}

	return nil
}

func isContextType(t reflect.Type) bool {
	return t == reflect.TypeFor[context.Context]()
}

func isErrorType(t reflect.Type) bool {
	return t.Implements(reflect.TypeFor[error]())
}

func lookupMethod(receiver reflect.Value, name string) (reflect.Value, bool) {
	if !receiver.IsValid() {
		return reflect.Value{}, false
	}

	target := receiver
	if receiver.Kind() != reflect.Ptr && receiver.CanAddr() {
		target = receiver.Addr()
	}

	method := target.MethodByName(name)

	return method, method.IsValid()
}

func methodValue(v reflect.Value, typ reflect.Type, method string) reflect.Value {
	if v.IsValid() {
		if m := v.MethodByName(method); m.IsValid() {
			return m
		}

		if v.CanAddr() {
			if m := v.Addr().MethodByName(method); m.IsValid() {
				return m
			}
		}
	}

	ptr := reflect.New(typ)
	if m := ptr.MethodByName(method); m.IsValid() {
		return m
	}

	if m := ptr.Elem().MethodByName(method); m.IsValid() {
		return m
	}

	return reflect.Value{}
}

func nodeChain(node *commandNode) []*commandNode {
	if node == nil {
		return nil
	}

	chain := make([]*commandNode, 0)
	for current := node; current != nil; current = current.Parent {
		chain = append(chain, current)
	}

	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return chain
}

func nodeHasAddressableValue(node *commandNode) bool {
	return node != nil && node.Value.IsValid() &&
		node.Value.Kind() == reflect.Struct && node.Value.CanAddr()
}

func nodeInstance(node *commandNode) (reflect.Value, error) {
	if nodeHasAddressableValue(node) {
		return node.Value, nil
	}

	if node == nil || node.Type == nil {
		return reflect.Value{}, nil
	}

	inst := reflect.New(node.Type).Elem()

	err := copySubcommandFuncs(inst, node)
	if err != nil {
		return reflect.Value{}, err
	}

	return inst, nil
}

func parseFunc(v reflect.Value) (*commandNode, error) {
	typ := v.Type()
	if typ.Kind() != reflect.Func {
		return nil, fmt.Errorf("expected func, got %v", typ.Kind())
	}

	err := validateFuncType(typ)
	if err != nil {
		return nil, err
	}

	name := functionName(v)
	if name == "" {
		return nil, errors.New("unable to determine function name")
	}

	return &commandNode{
		Name:        camelToKebab(name),
		Func:        v,
		Subcommands: make(map[string]*commandNode),
	}, nil
}

func parseStruct(t any) (*commandNode, error) {
	v, typ, err := resolveStructValue(t)
	if err != nil {
		return nil, err
	}

	if err := validateZeroFields(v, typ); err != nil {
		return nil, err
	}

	node := createCommandNode(v, typ)
	applyRunMethodDoc(node, typ)
	applyNameAndDescription(node, v, typ)

	if err := parseSubcommandFields(node, v, typ); err != nil {
		return nil, err
	}

	return node, nil
}

// parseSubcommandField parses a single subcommand field.
func parseSubcommandField(field reflect.StructField) (*commandNode, error) {
	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	if fieldType.Kind() == reflect.Func {
		err := validateFuncType(field.Type)
		if err != nil {
			return nil, err
		}

		return &commandNode{
			Func:        reflect.Zero(field.Type),
			Subcommands: make(map[string]*commandNode),
		}, nil
	}

	zeroVal := reflect.New(fieldType).Interface()

	return parseStruct(zeroVal)
}

// parseSubcommandFields parses all subcommand fields and adds them to the node.
func parseSubcommandFields(node *commandNode, v reflect.Value, typ reflect.Type) error {
	for i := range typ.NumField() {
		field := typ.Field(i)

		opts, err := tagOptionsForField(v, field)
		if err != nil {
			return err
		}

		if opts.Kind != TagKindSubcommand {
			continue
		}

		subNode, err := parseSubcommandField(field)
		if err != nil {
			return err
		}

		subNode.Parent = node
		subNode.Name = opts.Name
		node.Subcommands[subNode.Name] = subNode
	}

	return nil
}

func parseTagParts(opts *TagOptions, tag string) {
	parts := strings.SplitSeq(tag, ",")
	for p := range parts {
		applyTagPart(opts, strings.TrimSpace(p))
	}
}

// --- Parsing targets ---

func parseTarget(t any) (*commandNode, error) {
	if t == nil {
		return nil, errors.New("nil target")
	}

	v := reflect.ValueOf(t)
	if v.Kind() == reflect.Func {
		return parseFunc(v)
	}

	return parseStruct(t)
}

func printCommandHelp(node *commandNode) {
	binName := binaryName()
	if node.Type == nil {
		printSimpleHelp(binName, node)
		return
	}

	usageLine, err := buildUsageLine(node)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Usage: %s %s\n\n", binName, usageLine)

	printDescription(node.Description)
	printSubcommands(node.Subcommands)

	flags, err := collectFlagHelpChain(node)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	printFlags(flags)
}

func printCommandSummary(node *commandNode, indent string) {
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
		if sub != nil {
			printCommandSummary(sub, indent+"  ")
		}
	}
}

func printDescription(desc string) {
	if desc != "" {
		fmt.Println(desc)
		fmt.Println()
	}
}

func printFlags(flags []flagHelp) {
	if len(flags) == 0 {
		return
	}

	fmt.Println("Flags:")

	for _, item := range flags {
		name := formatFlagName(item)
		fmt.Printf("  %-24s %s\n", name, item.Usage)
	}
}

func printSimpleHelp(binName string, node *commandNode) {
	fmt.Printf("Usage: %s %s\n\n", binName, node.Name)

	if node.Description != "" {
		fmt.Println(node.Description)
	}
}

func printSubcommands(subs map[string]*commandNode) {
	if len(subs) == 0 {
		return
	}

	fmt.Println("Subcommands:")

	for name, sub := range subs {
		fmt.Printf("  %-20s %s\n", name, sub.Description)
	}

	fmt.Println()
}

func printTargOptions(opts RunOptions) {
	var flags []string
	if !opts.DisableCompletion {
		flags = append(flags, "  --completion [bash|zsh|fish]")
	}

	if !opts.DisableHelp {
		flags = append(flags, "  --help")
	}

	if !opts.DisableTimeout {
		flags = append(flags, "  --timeout <duration>")
	}

	if len(flags) > 0 {
		fmt.Println("\nTarg options:")

		for _, f := range flags {
			fmt.Println(f)
		}
	}
}

func printUsage(nodes []*commandNode, opts RunOptions) {
	fmt.Printf("Usage: %s <command> [args]\n", binaryName())
	fmt.Println("\nAvailable commands:")

	for _, node := range nodes {
		printCommandSummary(node, "  ")
	}

	printTargOptions(opts)
}

func resolvePlaceholder(opts TagOptions, kind reflect.Kind) string {
	if opts.Enum != "" {
		return fmt.Sprintf("{%s}", opts.Enum)
	}

	if opts.Placeholder != "" {
		return opts.Placeholder
	}

	switch kind {
	case reflect.String:
		return "<string>"
	case reflect.Int:
		return "<int>"
	case reflect.Bool:
		return "[flag]"
	default:
		return ""
	}
}

// resolveStructValue extracts the reflect.Value and Type from a target.
func resolveStructValue(t any) (reflect.Value, reflect.Type, error) {
	if t == nil {
		return reflect.Value{}, nil, errors.New("nil target")
	}

	v := reflect.ValueOf(t)
	typ := v.Type()

	if typ.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}, nil, errors.New("nil pointer target")
		}

		typ = typ.Elem()
		v = v.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return reflect.Value{}, nil, fmt.Errorf("expected struct, got %v", typ.Kind())
	}

	return v, typ, nil
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

func runCommand(
	ctx context.Context,
	node *commandNode,
	inst reflect.Value,
	_ []string,
	_ int,
) error {
	if node == nil {
		return nil
	}

	err := callMethod(ctx, inst, "Run")

	return err
}

func runPersistentHooks(ctx context.Context, chain []commandInstance, methodName string) error {
	for _, inst := range chain {
		if inst.node == nil || inst.node.Type == nil {
			continue
		}

		err := callMethod(ctx, inst.value, methodName)
		if err != nil {
			return err
		}
	}

	return nil
}

func skipShortExpansion(arg string, longInfo map[string]bool) bool {
	if arg == "--" || strings.HasPrefix(arg, "--") {
		return true
	}

	if len(arg) <= 2 || !strings.HasPrefix(arg, "-") {
		return true
	}

	if strings.Contains(arg, "=") {
		return true
	}

	group := strings.TrimPrefix(arg, "-")

	return len(group) <= 1 || longInfo[group]
}

// --- Tag options ---

func tagOptionsForField(inst reflect.Value, field reflect.StructField) (TagOptions, error) {
	tag := field.Tag.Get("targ")

	opts := TagOptions{
		Kind: TagKindFlag,
		Name: strings.ToLower(field.Name),
	}
	if strings.TrimSpace(tag) == "" {
		overridden, err := applyTagOptionsOverride(inst, field, opts)
		if err != nil {
			return TagOptions{}, err
		}

		return overridden, nil
	}

	detectTagKind(&opts, tag, field.Name)
	parseTagParts(&opts, tag)

	overridden, err := applyTagOptionsOverride(inst, field, opts)
	if err != nil {
		return TagOptions{}, err
	}

	return overridden, nil
}

func tagOptionsInstance(node *commandNode) reflect.Value {
	if node == nil {
		return reflect.Value{}
	}

	if node.Value.IsValid() {
		return node.Value
	}

	if node.Type != nil {
		return reflect.New(node.Type).Elem()
	}

	return reflect.Value{}
}

func tagOptionsMethod(inst reflect.Value) reflect.Value {
	if !inst.IsValid() {
		return reflect.Value{}
	}

	target := inst
	if inst.Kind() != reflect.Ptr {
		if inst.CanAddr() {
			target = inst.Addr()
		}
	}

	return target.MethodByName("TagOptions")
}

// validateFieldValue checks that a field is properly zero-valued.
func validateFieldValue(fieldVal reflect.Value, opts TagOptions, typeName, fieldName string) error {
	if opts.Kind == TagKindSubcommand {
		if fieldVal.Kind() == reflect.Func || fieldVal.IsZero() {
			return nil
		}

		return fmt.Errorf(
			"command %s must not prefill subcommand %s; use default tags instead",
			typeName,
			fieldName,
		)
	}

	if !fieldVal.IsZero() {
		return fmt.Errorf(
			"command %s must be zero value; use default tags instead of prefilled fields",
			typeName,
		)
	}

	return nil
}

func validateFuncType(typ reflect.Type) error {
	if typ.NumIn() > 1 {
		return errors.New("function command must be niladic or accept context")
	}

	if typ.NumIn() == 1 && !isContextType(typ.In(0)) {
		return errors.New("function command must accept context.Context")
	}

	if typ.NumOut() == 0 {
		return nil
	}

	if typ.NumOut() == 1 && isErrorType(typ.Out(0)) {
		return nil
	}

	return errors.New("function command must return only error")
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

func validateMethodInputs(
	ctx context.Context,
	method reflect.Value,
	name string,
) ([]reflect.Value, error) {
	mtype := method.Type()
	if mtype.NumIn() > 1 {
		return nil, fmt.Errorf("%s must accept context.Context or no args", name)
	}

	if mtype.NumIn() == 0 {
		return nil, nil
	}

	if !isContextType(mtype.In(0)) {
		return nil, fmt.Errorf("%s must accept context.Context", name)
	}

	return []reflect.Value{reflect.ValueOf(ctx)}, nil
}

func validateMethodOutputs(method reflect.Value, name string) error {
	mtype := method.Type()
	if mtype.NumOut() > 1 {
		return fmt.Errorf("%s must return only error", name)
	}

	if mtype.NumOut() == 1 && !isErrorType(mtype.Out(0)) {
		return fmt.Errorf("%s must return only error", name)
	}

	return nil
}

func validateTagOptionsSignature(method reflect.Value) error {
	mtype := method.Type()
	if mtype.NumIn() != 2 || mtype.NumOut() != 2 {
		return errors.New(
			"TagOptions must accept (string, TagOptions) and return (TagOptions, error)",
		)
	}

	if mtype.In(0).Kind() != reflect.String || mtype.In(1) != reflect.TypeFor[TagOptions]() {
		return errors.New("TagOptions must accept (string, TagOptions)")
	}

	if mtype.Out(0) != reflect.TypeFor[TagOptions]() || !isErrorType(mtype.Out(1)) {
		return errors.New("TagOptions must return (TagOptions, error)")
	}

	return nil
}

// validateZeroFields ensures fields are zero-valued (defaults should use tags).
func validateZeroFields(v reflect.Value, typ reflect.Type) error {
	for i := range typ.NumField() {
		field := typ.Field(i)

		opts, err := tagOptionsForField(v, field)
		if err != nil {
			return err
		}

		if err := validateFieldValue(v.Field(i), opts, typ.Name(), field.Name); err != nil {
			return err
		}
	}

	return nil
}
